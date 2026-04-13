import grpc
import inference_pb2
import inference_pb2_grpc

def run():
    print("Connecting to SRE-Pilot Engine [localhost:50051]...")
    channel = grpc.insecure_channel('localhost:50051')
    stub = inference_pb2_grpc.InferenceEngineStub(channel)
    
    # IMPORTANT: We use 128 tokens to match our C++ sequence_length
    # Sending 7000 tokens will now result in the engine only looking 
    # at the first 128, or potentially a memory mismatch error.
    test_tokens = [i for i in range(128)] 

    request = inference_pb2.InferenceRequest(
        model_id="classifier",
        tokens=test_tokens
    )
    
    print(f"Sending sequence of {len(request.tokens)} tokens...")
    
    try:
        response = stub.RunInference(request)
        
        # --- THE FIX: PRINT THE RESULTS ---
        print("\n" + "="*30)
        print("   INFERENCE ENGINE RESULT")
        print("="*30)
        
        # output_tokens is a 'repeated' field, so it behaves like a list
        for token in response.output_tokens:
            print(f"Prediction (Class): {token}")
            
        print(f"Model Confidence:    {response.confidence * 100:.2f}%")
        print(f"Is Result Stale?:    {response.is_stale}")
        print("="*30)

    except grpc.RpcError as e:
        print(f"CRITICAL: gRPC Failed -> {e.code()}: {e.details()}")

if __name__ == '__main__':
    run()