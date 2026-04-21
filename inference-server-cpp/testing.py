import grpc
import inference_pb2
import inference_pb2_grpc
import time

def run_test():
    # 1. Connect to the C++ Server
    channel = grpc.insecure_channel('localhost:50051')
    stub = inference_pb2_grpc.InferenceEngineStub(channel)

    # 2. Prepare a "Real World" log line
    test_log = "FATAL: connection to database failed: password authentication failed for user 'postgres'"
    
    print(f"📡 Sending log: '{test_log}'")
    
    try:
        # 3. Measure time for "Round Trip" (Go will eventually do this)
        start_time = time.time()
        
        response = stub.RunInference(inference_pb2.InferenceRequest(
            log_line=test_log,
            model_id="embedder"
        ))
        
        end_time = time.time()
        
        # 4. VERIFICATION
        vector = list(response.embedding)
        vector_len = len(vector)
        
        print("-" * 50)
        print(f"✅ RESPONSE RECEIVED in {(end_time - start_time)*1000:.2f}ms")
        print(f"📊 Vector Dimensions: {vector_len}")
        
        if vector_len == 384:
            print("💎 SUCCESS: This is a valid MiniLM Semantic Fingerprint.")
            # Print the first 5 numbers just to see the "DNA"
            print(f"📍 Sample (First 5 dims): {vector[:5]}")
        else:
            print(f"⚠️ WARNING: Expected 384 dims, got {vector_len}")

    except Exception as e:
        print(f"❌ FAILED to connect: {e}")

if __name__ == "__main__":
    run_test()