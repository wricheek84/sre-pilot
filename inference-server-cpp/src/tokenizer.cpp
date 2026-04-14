#include "tokenizer.h"
#include <fstream>
#include <iostream>
#include <stdexcept> 

Tokenizer::Tokenizer(const std::string & vocab_path) {
    std::ifstream infile(vocab_path);
    if (!infile.is_open()) {
        
        throw std::runtime_error("❌ Critical Error: Could not find vocab file at " + vocab_path);
    }

    
    vocab.reserve(31000);

    std::string line;
    int32_t index = 0;
    while (std::getline(infile, line)) {
        
        if (!line.empty() && line.back() == '\r') {
            line.pop_back();
        }

        if (!line.empty()) {
            vocab[line] = index;
        }
        index++; 
    } 

    infile.close();
    std::cout << "✅ Loaded " << vocab.size() << " tokens into the C++ Engine." << std::endl;  
}