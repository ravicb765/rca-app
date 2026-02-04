import logging
import os
from flask import Flask, request, jsonify
from rca_engine import RCAEngine
from train import train_from_csv

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Initialize the analyzer
analyzer = RCAEngine()

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "healthy"}), 200

@app.route('/analyze', methods=['POST'])
def analyze():
    try:
        payload = request.get_json(force=True)
        if not payload:
            return jsonify({"error": "Missing JSON payload"}), 400

        # Perform analysis
        result = analyzer.analyze(payload)

        return jsonify({
            'application_id': payload.get('application_id'),
            'analysis': result
        })
    except Exception as e:
        logger.error(f"Error during analysis: {str(e)}", exc_info=True)
        return jsonify({"error": "Internal server error"}), 500

@app.route('/retrain', methods=['POST'])
def retrain():
    if 'file' not in request.files:
        return jsonify({"error": "No file part"}), 400
    
    file = request.files['file']
    if file.filename == '':
        return jsonify({"error": "No selected file"}), 400

    try:
        stats = train_from_csv(analyzer, file)
        return jsonify({"status": "success", "stats": stats})
    except Exception as e:
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    app.run(host='0.0.0.0', port=port)
