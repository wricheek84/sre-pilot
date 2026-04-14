#include "tokenizer.h"
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <sstream>
#include <algorithm>
#include <cstdint>

Tokenizer::Tokenizer(const std::string& vocab_path) {
    std::ifstream infile(vocab_path);
    if (!infile.is_open()) {
        throw std::runtime_error("Critical Error: Could not find vocab file at " + vocab_path);
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
    std::cout << "Loaded " << vocab.size() << " tokens into the C++ Engine." << std::endl;
}

std::vector<int32_t> Tokenizer::tokenize(const std::string& text, size_t max_length) {
    std::vector<int32_t> ids;

    ids.push_back(101);

    std::stringstream ss(text);
    std::string word;

    while (ss >> word) {
        std::transform(word.begin(), word.end(), word.begin(),
                       [](unsigned char c) { return std::tolower(c); });

        wordpiece_tokenize(word, ids);
    }

    if (ids.size() >= max_length) {
        ids.resize(max_length - 1);
    }
    
    ids.push_back(102);

    while (ids.size() < max_length) {
        ids.push_back(0);
    }

    return ids;
}

void Tokenizer::wordpiece_tokenize(const std::string& word, std::vector<int32_t>& output) {
    bool is_bad = false;
    size_t start = 0;
    std::vector<int32_t> subword_ids;

    while (start < word.length()) {
        size_t end = word.length();
        int32_t cur_id = -1;

        while (start < end) {
            std::string substr = (start == 0)
                ? word.substr(start, end - start)
                : "##" + word.substr(start, end - start);

            auto it = vocab.find(substr);
            if (it != vocab.end()) {
                cur_id = it->second;
                break;
            }

            end--;
        }

        if (cur_id == -1) {
            is_bad = true;
            break;
        }

        subword_ids.push_back(cur_id);
        start = end;
    }

    if (is_bad) {
        output.push_back(100);
    } else {
        output.insert(output.end(), subword_ids.begin(), subword_ids.end());
    }
}