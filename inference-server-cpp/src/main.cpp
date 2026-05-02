#include <iostream>
#include <thread>
#include <vector>
#include <mutex>
#include <set>
#include <future> 
#include <memory> 
#include <grpcpp/grpcpp.h>
#include "tokenizer.h"
#include "inference.grpc.pb.h" 
#include "crow.h"
#include "simple_queue.hpp"
#include "thread_pool.hpp"
struct LogProjector {
    float mean[384];
    float components[3][384];

    
    bool init(const std::string& path) {
        std::ifstream f(path, std::ios::binary);
        if (!f.is_open()) return false;
        f.read((char*)mean, 384 * 4);
        f.read((char*)components, 3 * 384 * 4);
        return true;
    }

    
    std::vector<float> project(const std::vector<float>& input) {
        std::vector<float> coords(3, 0.0f);
        for (int k = 0; k < 3; k++) {
            for (int i = 0; i < 384; i++) {
                coords[k] += (input[i] - mean[i]) * components[k][i];
            }
        }
        return coords;
    }
};
SimpleQueue<InferenceRequest> order_queue;

class InferenceServiceImpl final : public inference::InferenceEngine::Service {
private:
    Tokenizer tokenizer;
    LogProjector& projector;

public:
    InferenceServiceImpl(const std::string& vocab_path, LogProjector& lp) : tokenizer(vocab_path), projector(lp) {}

    grpc::Status RunInference(grpc::ServerContext* context, const inference::InferenceRequest* request, inference::InferenceResponse* reply) override {
        if (order_queue.get_queue_depth() > 500) {
            return grpc::Status(grpc::StatusCode::RESOURCE_EXHAUSTED, "Server is overloaded");
        }

        std::string log_data = request->log_line();
        std::vector<int32_t> token_ids = tokenizer.tokenize(log_data);

        std::vector<int64_t> tokens_vec;
        tokens_vec.reserve(token_ids.size());
        for (int32_t id : token_ids) {
            tokens_vec.push_back(static_cast<int64_t>(id));
        }

        std::vector<int64_t> masks_vec(tokens_vec.size(), 1);

        std::cout << "Received request for model: " << request->model_id() << " | Tokens: " << tokens_vec.size() << std::endl;

        auto prom = std::make_shared<std::promise<InferenceResult>>();
        std::future<InferenceResult> fut = prom->get_future();

        order_queue.push({
            request->model_id(), 
            tokens_vec, 
            masks_vec, 
            std::chrono::steady_clock::now(),
            prom 
        });

        try {
            if (fut.wait_for(std::chrono::seconds(5)) == std::future_status::timeout) {
                return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Inference took too long");
            }

            InferenceResult result = fut.get();
            auto coords = projector.project(result.embedding);
            reply->set_x(coords[0]);
            reply->set_y(coords[1]);
            reply->set_z(coords[2]);
            
           
            for (float val : result.embedding) {
                reply->add_embedding(val);
            }
            reply->set_is_stale(result.is_stale);
            
        } catch (const std::exception& e) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Worker thread failure");
        }

        return grpc::Status::OK;
    }
};

int main() {
    unsigned int n = std::thread::hardware_concurrency();
    std::cout << "Starting " << n << " worker threads." << std::endl;

    
    std::vector<std::pair<std::string, std::wstring>> models = {
        {"embedder", L"C:\\Users\\wrich\\\\sre-pilot\\inference-server-cpp\\onnx\\embedder\\model_quantized.onnx"}
    };

    ThreadPool pool(n, order_queue, models);

    std::thread dashboard_thread([&pool]() {
        crow::SimpleApp app;
        std::mutex mtx;
        std::set<crow::websocket::connection*> users;

        CROW_WEBSOCKET_ROUTE(app, "/ws")
            .onopen([&](crow::websocket::connection& conn) {
                std::lock_guard<std::mutex> lock(mtx);
                users.insert(&conn);
            })
            .onclose([&](crow::websocket::connection& conn, const std::string& reason, uint16_t code) {
                std::lock_guard<std::mutex> lock(mtx);
                users.erase(&conn);
            });

        std::thread broadcaster([&]() {
            while (true) {
                std::this_thread::sleep_for(std::chrono::milliseconds(100));
                
                auto p = pool.get_percentiles();
                auto snap = pool.get_and_reset_telemetry();
                
                crow::json::wvalue x;
                x["queue_peak"] = snap.queue_peak;
                x["tasks_processed"] = snap.tasks_processed;
                x["worker_peak"] = snap.worker_peak;
                x["worker_active_time_ns"] = snap.worker_active_time_ns;
                x["p50_latency"] = p.p50;
                x["p99_latency"] = p.p99;
                x["total_batches"] = pool.get_total_batches();
                
                // Dashboard Telemetry: showing a sample of the vector
                x["latest_vector_sample"] = (double)pool.get_latest_val();

                std::string msg = x.dump();
                
                std::lock_guard<std::mutex> lock(mtx);
                for (auto u : users) {
                    u->send_text(msg);
                }
            }
        });
        broadcaster.detach();

        app.port(8080).multithreaded().run();
    });
    dashboard_thread.detach();
    LogProjector projector;
    if (!projector.init("C:\\Users\\wrich\\sre-pilot\\inference-server-cpp\\data\\projection.bin")) {
        std::cerr << "CRITICAL: Could not load projection.bin! Check the path." << std::endl;
        return 1;
    }

    std::string server_address("0.0.0.0:50051");
    InferenceServiceImpl service("C:\\Users\\wrich\\sre-pilot\\inference-server-cpp\\onnx\\embedder\\vocab.txt", projector);
    grpc::ServerBuilder builder;
    builder.AddListeningPort(server_address, grpc::InsecureServerCredentials());
    builder.RegisterService(&service);
    
    std::unique_ptr<grpc::Server> server(builder.BuildAndStart());
    std::cout << "gRPC Inference Server listening on " << server_address << std::endl;
    server->Wait();

    return 0;
}