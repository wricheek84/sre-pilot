from huggingface_hub import hf_hub_download

print("Downloading the vocabulary file...")
hf_hub_download(
    repo_id="Xenova/distilbert-base-uncased-finetuned-sst-2-english",
    filename="vocab.txt",
    local_dir="."
)
print("Done! Check your folder for 'vocab.txt'.")