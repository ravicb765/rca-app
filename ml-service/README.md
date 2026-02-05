# RCA-App ML Service

The ML Service provides intelligence to the observability platform, enabling proactive detection and explanation of incidents.

## Features

- **Anomaly Detection**: Uses Isolation Forest to detect statistical outliers in service metrics (CPU, Latency, Errors).
- **Incident Classification**: Classifies incidents (e.g., "Database Slowdown", "Memory Leak") using Random Forest.
- **Forecasting**: Predicts future metric trends using Facebook Prophet / LSTM to alert on deviations.
- **Root Cause Explainer**: An LLM-powered RAG (Retrieval-Augmented Generation) system that explains incidents in plain English.

## Setup

### Requirements

- Python 3.9+
- `pip`

### Install Dependencies

```bash
pip install -r requirements.txt
```

### Run Locally

```bash
python main.py
```

Service listens on `0.0.0.0:5000`.

## API Endpoints

- `POST /predict`: Detect anomalies in a given metric vector.
- `POST /forecast`: Generate a time-series forecast.
- `POST /explain`: Generate a text explanation for an incident context.

## Models

Models are trained offline and serialized using `joblib`. 
- `models/isolation_forest.joblib`
- `models/classifier.joblib`

To retrain models, use the provided scripts in `tests/` or create a new training pipeline.
