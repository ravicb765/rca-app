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
        self._train_dummy_models()

    def _train_dummy_models(self):
        # Dummy normal data for Anomaly Detection (CPU, Mem, Latency, ErrorRate, RequestRate)
        rng = np.random.RandomState(42)
        X_normal = rng.rand(100, 5) * [40, 30, 40, 0.1, 50] + [20, 40, 10, 0, 10]
        self.anomaly_detector.train(X_normal.tolist())

        # Dummy incident data for Classifier
        # Features: [CPU, Mem, Latency, ErrorRate, RequestRate]
        X_incidents = [
            [30, 40, 500, 0.1, 100], # Database Slowdown
            [30, 95, 40, 0.05, 80],  # Memory Leak
            [30, 40, 50, 5.0, 70],   # Network Issue
            [90, 40, 200, 0.05, 500], # High Traffic
            [30, 40, 50, 10.0, 50],  # Dependency Failure
        ]
        y_incidents = ["database_slowdown", "memory_leak", "network_issue", "high_traffic", "dependency_failure"]
        
        X_train = np.array(X_incidents * 20)
        y_train = y_incidents * 20
        self.classifier.train(X_train, y_train)

    def analyze(self, incident_data):
        metrics_dict = incident_data.get('metrics', {})
        feature_vector = [
            float(metrics_dict.get('cpu', 0)),
            float(metrics_dict.get('memory', 0)),
            float(metrics_dict.get('latency', 0)),
            float(metrics_dict.get('error_rate', 0)),
            float(metrics_dict.get('request_rate', 0))
        ]

        is_anomaly = self.anomaly_detector.detect(feature_vector) == -1

        if not is_anomaly:
            return {
                "is_anomaly": False,
                "root_cause": "normal_operation",
                "confidence": 1.0,
                "reasoning": "Application is operating within normal parameters.",
                "remediation": []
            }

        # 2. Classify Incident
        cause = self.classifier.predict(feature_vector)
        
        # 3. LLM-based RCA (Simulated/Stub)
        # In production, this would call GPT-4 or a local LLM via LangChain/OpenAI API
        result = self._call_llm_analyzer(cause, feature_vector)
        
        # Add classification confidence
        probs = self.classifier.model.predict_proba([feature_vector])[0]
        result["confidence"] = float(max(probs))
        
        return result

    def _call_llm_analyzer(self, cause, metrics):
        """Stub for LLM-based Root Cause Analysis"""
        cpu, mem, lat, err, req = metrics
        
        # This simulates the output of an LLM processing the metrics and classification
        reasons = {
            "database_slowdown": {
                "reasoning": f"Latency spike ({lat}ms) detected with normal CPU. Correlation with DB pool metrics suggests lock contention.",
                "remediation": ["Check for long-running transactions", "Verify DB connection pool usage"]
            },
            "memory_leak": {
                "reasoning": f"Continuous memory growth ({mem}%) observed. Heap histograms indicate large retention in cache objects.",
                "remediation": ["Run memory profiler", "Check for cache eviction leaks"]
            },
            "network_issue": {
                "reasoning": f"Packet loss and connection timeouts detected. High error rate ({err}%) correlates with network saturation.",
                "remediation": ["Check network policies", "Verify upstream service availability"]
            },
            "high_traffic": {
                "reasoning": f"Request rate ({req} req/s) exceeded 3x baseline. CPU ({cpu}%) is saturated due to volume.",
                "remediation": ["Horizontal scale replicas", "Check rate limiting configurations"]
            },
            "dependency_failure": {
                "reasoning": f"Upstream service 'auth-api' is returning 5xx. Local error rate ({err}%) is caused by timeout propagation.",
                "remediation": ["Check health of 'auth-api'", "Implement circuit breaking"]
            }
        }
        
        analysis = reasons.get(cause, {
            "reasoning": "Anomalous behavior detected. Requires manual investigation.",
            "remediation": ["Analyze logs and metrics", "Check recent deployments"]
        })
        
        return {
            "is_anomaly": True,
            "root_cause": cause,
            "reasoning": analysis["reasoning"],
            "remediation": analysis["remediation"]
        }

    def train(self, normal_data, incident_features, incident_labels):
        """
        Retrain models with provided data.
        """
        if normal_data:
            self.anomaly_detector.train(normal_data)
        
        if incident_features and incident_labels:
            self.classifier.train(incident_features, incident_labels)