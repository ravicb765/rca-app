import pandas as pd
import logging

logger = logging.getLogger(__name__)

def train_from_csv(engine, csv_source):
    """
    Train the engine from a CSV file or file-like object.
    Expected columns: cpu, memory, latency, error_rate, label
    
    label='normal' -> used for anomaly detector training
    label!='normal' -> used for incident classifier training
    """
    try:
        df = pd.read_csv(csv_source)
        
        required_cols = ['cpu', 'memory', 'latency', 'error_rate', 'label']
        if not all(col in df.columns for col in required_cols):
            raise ValueError(f"CSV must contain columns: {required_cols}")

        # Split data
        normal_df = df[df['label'] == 'normal']
        incident_df = df[df['label'] != 'normal']

        feature_cols = ['cpu', 'memory', 'latency', 'error_rate']
        
        normal_data = normal_df[feature_cols].values.tolist()
        incident_features = incident_df[feature_cols].values.tolist()
        incident_labels = incident_df['label'].values.tolist()

        engine.train(normal_data, incident_features, incident_labels)
        
        return {
            "normal_samples": len(normal_data),
            "incident_samples": len(incident_features)
        }
    except Exception as e:
        logger.error(f"Failed to train from CSV: {e}")
        raise e

if __name__ == "__main__":
    import sys
    from rca_engine import RCAEngine
    
    logging.basicConfig(level=logging.INFO)
    
    if len(sys.argv) < 2:
        print("Usage: python train.py <data.csv>")
        sys.exit(1)
        
    engine = RCAEngine()
    stats = train_from_csv(engine, sys.argv[1])
    print(f"Training successful: {stats}")