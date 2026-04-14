import grpc
import inference_pb2
import inference_pb2_grpc

def run():
    print("Connecting to SRE-Pilot Engine [localhost:50051]...")
    channel = grpc.insecure_channel('localhost:50051')
    stub = inference_pb2_grpc.InferenceEngineStub(channel)
    
    # We now send a raw string. The C++ Tokenizer handles the 128 tokens for us.
    test_log = "ERROR: Database connection timeout on port 5432. Retrying..."

    request = inference_pb2.InferenceRequest(
        model_id="classifier",
        log_line=test_log
    )
    
    print(f"Sending log line: {test_log}")
    
    try:
        response = stub.RunInference(request)
        
        print("\n" + "="*30)
        print("   INFERENCE ENGINE RESULT")
        print("="*30)
        
        for token in response.output_tokens:
            print(f"Prediction (Class): {token}")
            
        print(f"Model Confidence:    {response.confidence * 100:.2f}%")
        print(f"Is Result Stale?:    {response.is_stale}")
        print("="*30)

    except grpc.RpcError as e:
        print(f"CRITICAL: gRPC Failed -> {e.code()}: {e.details()}")

if __name__ == '__main__':
    run()