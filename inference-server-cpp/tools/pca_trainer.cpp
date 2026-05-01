#include <iostream>
#include <fstream>
#include <vector>
#include <cmath>

using namespace std;

int main() {
    const int N = 2000;
    const int D = 384;
    
    ifstream f("logs.bin", ios::binary);
    if (!f) {
        cout << "Error: logs.bin not found" << endl;
        return 1;
    }
    
    vector<float> data(N * D);
    f.read((char*)data.data(), N * D * 4);
    f.close();

    // Calculate Mean
    vector<float> m(D, 0);
    for (int i = 0; i < N; i++) {
        for (int j = 0; j < D; j++) m[j] += data[i * D + j] / N;
    }

    // Center Data
    for (int i = 0; i < N; i++) {
        for (int j = 0; j < D; j++) data[i * D + j] -= m[j];
    }

    vector<float> res(D * 4);
    for (int j = 0; j < D; j++) res[j] = m[j];

    // Power Iteration for Top 3 Eigenvectors
    for (int k = 0; k < 3; k++) {
        vector<float> v(D, 0.1f);
        for (int it = 0; it < 15; it++) {
            vector<float> next(D, 0);
            for (int i = 0; i < N; i++) {
                float dot = 0;
                for (int j = 0; j < D; j++) dot += data[i * D + j] * v[j];
                for (int j = 0; j < D; j++) next[j] += dot * data[i * D + j];
            }
            float norm = 0;
            for (float x : next) norm += x * x;
            norm = sqrt(norm);
            for (int j = 0; j < D; j++) v[j] = next[j] / (norm + 1e-9f);
        }
        for (int i = 0; i < N; i++) {
            float dot = 0;
            for (int j = 0; j < D; j++) dot += data[i * D + j] * v[j];
            for (int j = 0; j < D; j++) data[i * D + j] -= dot * v[j];
        }
        for (int j = 0; j < D; j++) res[(k + 1) * D + j] = v[j];
    }

    ofstream o("projection.bin", ios::binary);
    o.write((char*)res.data(), res.size() * 4);
    o.close();

    cout << "SUCCESS: projection.bin generated" << endl;
    return 0;
}
