from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/analyze', methods=['POST'])
def analyze():
    payload = request.get_json(force=True)
    # Placeholder ML analysis - echo useful fields
    return jsonify({
        'application_id': payload.get('application_id'),
        'summary': 'stub analysis',
        'input': payload.get('metrics')
    })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
