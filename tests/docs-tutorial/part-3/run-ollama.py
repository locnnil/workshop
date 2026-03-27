import requests

# Check if Ollama is running
response = requests.get('http://localhost:11434/api/version')
print(f"Ollama version: {response.json()['version']}")

# Generate text with the tinyllama model, installed in Part 1
data = {
    "model": "tinyllama",
    "prompt": "Why is the sky blue?",
    "stream": False
}
response = requests.post('http://localhost:11434/api/generate', json=data)
print(response.json()['response'])
