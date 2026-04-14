#ifndef TOKENIZER_H
#define TOKENIZER_H

#include <string>
#include <vector>
#include <unordered_map>

class Tokenizer {
public:
    
    Tokenizer(const std::string& vocab_path);

    
    std::vector<int32_t> tokenize(const std::string& text, size_t max_length = 128);

private:
    
    std::unordered_map<std::string, int32_t> vocab;
    void wordpiece_tokenize(const std::string& word, std::vector<int32_t>& output);
};

#endif