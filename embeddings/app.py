from flask import Flask, request, jsonify
from sentence_transformers import SentenceTransformer
import logging

app = Flask(__name__)
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Загружаем модель при старте
logger.info("Loading sentence-transformers/all-MiniLM-L6-v2...")
model = SentenceTransformer('all-MiniLM-L6-v2')
logger.info("Model loaded successfully")


@app.route('/embed', methods=['POST'])
def embed():
    """
    Генерирует 384-мерный эмбединг для текста.
    
    Ожидает JSON: {"text": "какой-то текст"}
    Возвращает: {"embedding": [0.1, 0.2, ..., 0.3]}
    """
    try:
        data = request.get_json()
        
        if not data or 'text' not in data:
            return jsonify({"error": "Missing 'text' field"}), 400
        
        text = data['text']
        
        if not isinstance(text, str) or not text.strip():
            return jsonify({"error": "Text must be non-empty string"}), 400
        
        # Генерируем эмбединг
        embedding = model.encode(text).tolist()
        
        return jsonify({"embedding": embedding}), 200
    
    except Exception as e:
        logger.error(f"Error in /embed: {e}")
        return jsonify({"error": str(e)}), 500


@app.route('/health', methods=['GET'])
def health():
    """Health check для Docker"""
    return jsonify({"status": "healthy"}), 200


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8003, debug=False)
