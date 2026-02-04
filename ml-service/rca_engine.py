import numpy as np
import logging
from sklearn.ensemble import IsolationForest, RandomForestClassifier
from sklearn.preprocessing import LabelEncoder

logger = logging.getLogger(__name__)

class AnomalyDetector:
    def __init__(self):
        # Contamination is the expected proportion of outliers
        self.model = IsolationForest(contamination=0.05, random_state=42)
        self.is_fitted = False

    def train(self, data):
        """
        Train the anomaly detector on historical metric data.
        data: list of lists or 2D array of metrics [[cpu, mem, latency, error_rate], ...]
        """
        if not data:
            return
        self.model.fit(data)
        self.is_fitted = True
        logger.info("AnomalyDetector trained successfully.")

    def detect(self, metrics):
        """
        Returns -1 for anomaly, 1 for normal.
        metrics: list or 1D array [cpu, mem, latency, error_rate]
        """
        if not self.is_fitted:
            # Fallback if not trained: assume normal for safety
            return 1 
        return self.model.predict([metrics])[0]

class IncidentClassifier:
    def __init__(self):
        self.model = RandomForestClassifier(n_estimators=100, random_state=42)
        self.label_encoder = LabelEncoder()
        self.is_fitted = False

    def train(self, features, labels):
        """
        Train the classifier on historical incidents.
        features: 2D array of incident metrics
        labels: list of strings (e.g., 'db_slow', 'network_issue')
        """
        if len(features) == 0 or len(labels) == 0:
            return
        
        y = self.label_encoder.fit_transform(labels)
        self.model.fit(features, y)
        self.is_fitted = True
        logger.info("IncidentClassifier trained successfully.")

    def predict(self, metrics):
        """
        Predict the category of the incident.
        """
        if not self.is_fitted:
            return "unknown"
        
        pred_idx = self.model.predict([metrics])[0]
        return self.label_encoder.inverse_transform([pred_idx])[0]

class RCAEngine:
    def __init__(self):
        self.anomaly_detector = AnomalyDetector()
        self.classifier = IncidentClassifier()
        
        # Initialize with dummy data for demonstration
        # In production, this would load from a database or model file
        self._train_dummy_models()

    def _train_dummy_models(self):
        # Dummy normal data for Anomaly Detection (CPU, Mem, Latency, ErrorRate)
        # Normal: CPU 20-60%, Mem 40-70%, Latency 10-50ms, Error 0-0.1%
        rng = np.random.RandomState(42)
        X_normal = rng.rand(100, 4) * [40, 30, 40, 0.1] + [20, 40, 10, 0]
        self.anomaly_detector.train(X_normal.tolist())

        # Dummy incident data for Classifier
        # Features: [CPU, Mem, Latency, ErrorRate]
        X_incidents = [
            [95, 80, 200, 0.5], # High CPU/Mem -> Resource Exhaustion
            [20, 40, 1000, 0.1], # High Latency -> Database Slowdown
            [30, 40, 50, 5.0],   # High Error -> Network/Dependency
        ]
        y_incidents = ["resource_exhaustion", "database_slowdown", "network_issue"]
        
        # Duplicate to have enough samples for training
        X_train = np.array(X_incidents * 10)
        y_train = y_incidents * 10
        self.classifier.train(X_train, y_train)

    def analyze(self, incident_data):
        """
        Analyze current metrics to find anomalies and root causes.
        incident_data: dict containing 'metrics' key
        """
        metrics_dict = incident_data.get('metrics', {})
        # Extract feature vector in correct order: CPU, Mem, Latency, ErrorRate
        feature_vector = [
            float(metrics_dict.get('cpu', 0)),
            float(metrics_dict.get('memory', 0)),
            float(metrics_dict.get('latency', 0)),
            float(metrics_dict.get('error_rate', 0))
        ]

        # 1. Detect Anomaly
        is_anomaly = self.anomaly_detector.detect(feature_vector) == -1

        result = {
            "is_anomaly": is_anomaly,
            "root_cause": "normal_operation",
            "confidence": 1.0 if not is_anomaly else 0.0,
            "remediation": []
        }

        if is_anomaly:
            # 2. Classify Incident
            cause = self.classifier.predict(feature_vector)
            result["root_cause"] = cause
            
            # Simple confidence heuristic
            probs = self.classifier.model.predict_proba([feature_vector])[0]
            confidence = max(probs)
            result["confidence"] = float(confidence)
            result["remediation"] = self._get_remediation(cause)

        return result

    def _get_remediation(self, cause):
        remediations = {
            "resource_exhaustion": ["Scale up replicas", "Check for memory leaks", "Optimize CPU intensive tasks"],
            "database_slowdown": ["Check database locks", "Analyze slow queries", "Check connection pool"],
            "network_issue": ["Check upstream dependencies", "Verify network policies", "Check DNS resolution"],
            "unknown": ["Check application logs", "Investigate recent deployments"]
        }
        return remediations.get(cause, ["Investigate manually"])

    def train(self, normal_data, incident_features, incident_labels):
        """
        Retrain models with provided data.
        """
        if normal_data:
            self.anomaly_detector.train(normal_data)
        
        if incident_features and incident_labels:
            self.classifier.train(incident_features, incident_labels)