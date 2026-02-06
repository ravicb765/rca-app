import unittest
from unittest.mock import MagicMock, patch
import sys
import os

# Add parent dir to path to import modules
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from rca_engine import RCAEngine, AnomalyDetector, IncidentClassifier

class TestAnomalyDetector(unittest.TestCase):
    def setUp(self):
        self.detector = AnomalyDetector()

    def test_predict(self):
        # Mocking the internal model for unit testing without loading file
        self.detector.model = MagicMock()
        self.detector.model.predict.return_value = [-1] # Anomaly
        self.detector.model.decision_function.return_value = [-0.5]

        data = {"cpu": 0.9, "latency": 500}
        result = self.detector.detect([data])
        
        self.assertTrue(result['is_anomaly'])
        self.assertEqual(result['score'], -0.5)

class TestIncidentClassifier(unittest.TestCase):
    def setUp(self):
        self.classifier = IncidentClassifier()
    
    def test_classify(self):
        self.classifier.model = MagicMock()
        self.classifier.model.predict.return_value = ["database_slowdown"]
        self.classifier.model.predict_proba.return_value = [[0.8, 0.2]]
        self.classifier.model.classes_ = ["database_slowdown", "network_issue"]

        data = {"latency": 500, "db_connections": 100}
        result = self.classifier.classify([data])

        self.assertEqual(result['incident_type'], "database_slowdown")
        self.assertGreater(result['confidence'], 0.7)

if __name__ == '__main__':
    unittest.main()
