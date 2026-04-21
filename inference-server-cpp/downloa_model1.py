from huggingface_hub import hf_hub_download
import os

# The destination folder you just created
target_dir = "./onnx/embedder"
model_id = "Xenova/all-MiniLM-L6-v2"

print(f"Checking for folder: {target_dir}")
os.makedirs(target_dir, exist_ok=True)

# 1. Download the new Model
print("Downloading MiniLM Model (the Fingerprinter)...")
hf_hub_download(
    repo_id=model_id,
    filename="onnx/model_quantized.onnx",
    local_dir=target_dir,
    force_download=True 
)

# 2. Download the new Vocab
print("Downloading new vocab.txt...")
hf_hub_download(
    repo_id=model_id,
    filename="vocab.txt",
    local_dir=target_dir,
    force_download=True
)

print("\nSuccess. Check your folder—you should see two files in /onnx/embedder.")
