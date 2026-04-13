#pragma once
#include <vector>
#include <thread>
#include <iostream>
#include "simple_queue.hpp"
#include <chrono>
#include <onnxruntime_cxx_api.h>
#include "tensor.hpp"
#include <atomic>
#include <mutex>
#include <algorithm>
#include <unordered_map>
#include <string>
#include <cmath>
#include <future> 
#include <memory> 
#include <deque>
struct InferenceResult {
    int prediction;
    float confidence;
    bool is_stale;
};

struct InferenceRequest {
    std::string model_id; 
    std::vector<int64_t> tokens;
    std::vector<int64_t> masks;
    std::chrono::steady_clock::time_point arrival_time;
    std::shared_ptr<std::promise<InferenceResult>> promise;
};

struct TelemetrySnapshot {
    int queue_peak;
    int tasks_processed;
    int worker_peak;
    long long worker_active_time_ns;
    int dropped_tasks;
};

class ThreadPool {
private:
    struct TelemetryWindow {
        std::atomic<int> max_queue_depth{0};
        std::atomic<int> tasks_processed{0};
        std::atomic<int> active_worker_count_peak{0};
        std::atomic<long long> worker_active_time_ns{0};
        std::atomic<int> dropped_tasks{0};
    };

    std::vector<std::thread> workers;
    std::deque<double> latency_samples;
    std::mutex samples_mutex;
    Ort::Env env{ORT_LOGGING_LEVEL_FATAL, "InferenceServer"};
    std::unordered_map<std::string, std::unique_ptr<Ort::Session>> sessions;
    SimpleQueue<InferenceRequest>& queue;
    std::atomic<int> active_workers{0};
    std::atomic<int> total_processed{0};
    std::atomic<int> total_batches{0};
    std::atomic<long long> total_latency_ms{0};
    
    std::atomic<int> latest_prediction{0};
    std::atomic<float> latest_confidence{0.0f};
    
    TelemetryWindow telemetry;

   void worker_loop(int id) {
        while (true) {
            std::vector<InferenceRequest> requests = queue.pop_batch(32, 25);
            
            if (requests.empty()) continue; 
            
            // 1. Shutdown Check
            if(!requests[0].tokens.empty() && requests[0].tokens[0] == -1){
                std::cout << "Worker " << id << " shutting down!" << std::endl;
                break;
            }

            auto now_check = std::chrono::steady_clock::now();
            std::vector<int64_t> batch_tokens; 
            std::vector<int64_t> batch_masks; 
            std::vector<size_t> valid_indices; 

        
            for (size_t i = 0; i < requests.size(); ++i) {
                auto wait_time = std::chrono::duration_cast<std::chrono::milliseconds>(now_check - requests[i].arrival_time).count();
                
                if (wait_time > 100) {
                    telemetry.dropped_tasks.fetch_add(1, std::memory_order_relaxed);
                    if(requests[i].promise) {
                        try {
                            requests[i].promise->set_value({-1, 0.0f, true});
                        } catch (...) {} 
                    }
                } else {
                    // Only process tasks that haven't timed out
                    batch_tokens.insert(batch_tokens.end(), requests[i].tokens.begin(), requests[i].tokens.end());
                    batch_masks.insert(batch_masks.end(), requests[i].masks.begin(), requests[i].masks.end());
                    valid_indices.push_back(i);
                }
            }

            
            if (valid_indices.empty()) continue;

            try {
                
                int64_t actual_batch_size = static_cast<int64_t>(valid_indices.size());
                const int64_t sequence_length = 128;
                std::vector<int64_t> shape = { actual_batch_size, sequence_length };

                Ort::MemoryInfo memory_info = Ort::MemoryInfo::CreateCpu(OrtArenaAllocator, OrtMemTypeDefault);
                Ort::Value ids_tensor = Ort::Value::CreateTensor<int64_t>(memory_info, batch_tokens.data(), batch_tokens.size(), shape.data(), shape.size());
                Ort::Value mask_tensor = Ort::Value::CreateTensor<int64_t>(memory_info, batch_masks.data(), batch_masks.size(), shape.data(), shape.size());

                const char* input_names[] = {"input_ids", "attention_mask"};
                const char* output_names[] = {"logits"};
                Ort::Value input_tensors[] = {std::move(ids_tensor), std::move(mask_tensor)};
                
                active_workers++;
                auto start_infer = std::chrono::steady_clock::now();
                auto output_tensors = sessions.at(requests[0].model_id)->Run(Ort::RunOptions{nullptr}, input_names, input_tensors, 2, output_names, 1);
                auto end_infer = std::chrono::steady_clock::now();
                active_workers--;

                
                {
                    std::lock_guard<std::mutex> lock(samples_mutex);
                    latency_samples.push_back(std::chrono::duration<double, std::milli>(end_infer - start_infer).count());
                    if (latency_samples.size() > 1000){
                        latency_samples.pop_front();
                    }
                }
                telemetry.worker_active_time_ns.fetch_add(std::chrono::duration_cast<std::chrono::nanoseconds>(end_infer - start_infer).count(), std::memory_order_relaxed);
                telemetry.tasks_processed.fetch_add((int)actual_batch_size, std::memory_order_relaxed);
                total_processed += (int)actual_batch_size;
                total_batches++;

            
                float* floatarr = output_tensors.front().GetTensorMutableData<float>();

                for (int b = 0; b < actual_batch_size; ++b) {
                    float logit0 = floatarr[b * 2];
                    float logit1 = floatarr[b * 2 + 1];

                    int prediction = (logit1 > logit0) ? 1 : 0;
                    float max_val = std::max(logit0, logit1);
                    float sum_exp = std::exp(logit0 - max_val) + std::exp(logit1 - max_val);
                    float confidence = std::exp((prediction == 1 ? logit1 : logit0) - max_val) / sum_exp;
                    
                    latest_prediction.store(prediction, std::memory_order_relaxed);
                    latest_confidence.store(confidence, std::memory_order_relaxed);

                    
                    size_t original_index = valid_indices[b];
                    if (requests[original_index].promise) {
                        try {
                            requests[original_index].promise->set_value({prediction, confidence, false});
                        } catch (...) {}
                    }

                    if (b == 0) {
                        std::cout << "[WORKER " << id << "] Healthy Result: Class " << prediction 
                                  << " | Confidence: " << (confidence * 100.0f) << "%" << std::endl;
                    }
                }

            } catch (const std::exception& e) {
                std::cerr << "[WORKER " << id << "] ERROR: " << e.what() << std::endl;
                if(active_workers > 0) active_workers--;
            }
        }
    }

public:
    struct Percentiles { double p50; double p99; };

    ThreadPool(int num_threads, SimpleQueue<InferenceRequest>& q, 
               const std::vector<std::pair<std::string, std::wstring>>& models_to_load) : queue(q) {
        
        Ort::SessionOptions session_options;
        session_options.SetIntraOpNumThreads(1);
        
        for (const auto& item : models_to_load) {
            try {
                std::cout << "[INIT] Loading model '" << item.first << "'..." << std::endl;
                sessions[item.first] = std::make_unique<Ort::Session>(env, item.second.c_str(), session_options);
            } catch (const Ort::Exception& e) {
                std::cerr << "[CRITICAL] Failed to load " << item.first << ": " << e.what() << std::endl;
                exit(1);
            }
        }

        for (int i = 0; i < num_threads; ++i) {
            workers.emplace_back(&ThreadPool::worker_loop, this, i);
        }
    }

    ~ThreadPool() {
        for(size_t i = 0; i < workers.size(); i++){

            
            queue.push(InferenceRequest{"", {-1}, {0}, std::chrono::steady_clock::now(), nullptr});
        }
        for (auto& worker : workers) {
            if (worker.joinable()) worker.join();
        }
    }

    int get_active_workers() const { return active_workers.load(); }
    int get_total_processed() const { return total_processed.load(); }   
    int get_total_batches() const { return total_batches.load(); }
    
    int get_latest_prediction() const { return latest_prediction.load(); }
    float get_latest_confidence() const { return latest_confidence.load(); }

    Percentiles get_percentiles() {
        std::lock_guard<std::mutex> lock(samples_mutex);
        if (latency_samples.empty()) return {0.0, 0.0};
        
        std::vector<double> sorted(latency_samples.begin(), latency_samples.end());
        
        std::sort(sorted.begin(), sorted.end());
        return {sorted[(size_t)(sorted.size() * 0.50)], sorted[(size_t)(sorted.size() * 0.99)]};
        
    }

    TelemetrySnapshot get_and_reset_telemetry() {
        return {
            telemetry.max_queue_depth.exchange(0, std::memory_order_relaxed),
            telemetry.tasks_processed.exchange(0, std::memory_order_relaxed),
            telemetry.active_worker_count_peak.exchange(0, std::memory_order_relaxed),
            telemetry.worker_active_time_ns.exchange(0, std::memory_order_relaxed),
            telemetry.dropped_tasks.exchange(0, std::memory_order_relaxed)
        };
    }
};