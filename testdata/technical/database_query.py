#!/usr/bin/env python3
"""
Database Query Module - Handles complex SQL operations for analytics
"""

import psycopg2
from datetime import datetime, timedelta
import pandas as pd
import numpy as np

class DatabaseAnalytics:
    def __init__(self, connection_string):
        """Initialize database connection for analytics queries."""
        self.conn = psycopg2.connect(connection_string)
        self.cursor = self.conn.cursor()
    
    def get_quarterly_revenue(self, year, quarter):
        """
        Retrieve quarterly revenue data with year-over-year comparison.
        
        Args:
            year: Fiscal year (e.g., 2024)
            quarter: Quarter number (1-4)
        
        Returns:
            DataFrame with revenue analysis
        """
        query = """
        SELECT 
            DATE_TRUNC('month', transaction_date) as month,
            product_category,
            SUM(revenue) as total_revenue,
            COUNT(DISTINCT customer_id) as unique_customers,
            AVG(transaction_amount) as avg_transaction,
            SUM(revenue) - LAG(SUM(revenue)) OVER (ORDER BY DATE_TRUNC('month', transaction_date)) as month_over_month_change
        FROM sales_transactions
        WHERE EXTRACT(YEAR FROM transaction_date) = %s
            AND EXTRACT(QUARTER FROM transaction_date) = %s
        GROUP BY DATE_TRUNC('month', transaction_date), product_category
        ORDER BY month, total_revenue DESC;
        """
        
        self.cursor.execute(query, (year, quarter))
        results = self.cursor.fetchall()
        
        # Convert to DataFrame for analysis
        df = pd.DataFrame(results, columns=[
            'month', 'category', 'revenue', 'customers', 'avg_transaction', 'mom_change'
        ])
        
        # Calculate additional metrics
        df['revenue_per_customer'] = df['revenue'] / df['customers']
        df['growth_rate'] = (df['mom_change'] / df['revenue'].shift(1)) * 100
        
        return df
    
    def detect_anomalies(self, metric='revenue', threshold=2.5):
        """
        Detect statistical anomalies in financial metrics using z-score.
        
        Uses standard deviation to identify outliers that may indicate
        fraud, errors, or exceptional performance.
        """
        query = """
        WITH daily_metrics AS (
            SELECT 
                transaction_date,
                SUM(revenue) as daily_revenue,
                COUNT(*) as transaction_count
            FROM sales_transactions
            WHERE transaction_date >= CURRENT_DATE - INTERVAL '90 days'
            GROUP BY transaction_date
        ),
        stats AS (
            SELECT 
                AVG(daily_revenue) as mean_revenue,
                STDDEV(daily_revenue) as stddev_revenue
            FROM daily_metrics
        )
        SELECT 
            dm.transaction_date,
            dm.daily_revenue,
            dm.transaction_count,
            (dm.daily_revenue - s.mean_revenue) / s.stddev_revenue as z_score,
            CASE 
                WHEN ABS((dm.daily_revenue - s.mean_revenue) / s.stddev_revenue) > %s 
                THEN 'ANOMALY DETECTED'
                ELSE 'Normal'
            END as status
        FROM daily_metrics dm, stats s
        WHERE ABS((dm.daily_revenue - s.mean_revenue) / s.stddev_revenue) > %s
        ORDER BY z_score DESC;
        """
        
        self.cursor.execute(query, (threshold, threshold))
        anomalies = self.cursor.fetchall()
        
        if anomalies:
            print(f"⚠️ Found {len(anomalies)} anomalies in {metric} data")
            for date, revenue, count, z_score, status in anomalies:
                print(f"  {date}: ${revenue:,.2f} (z-score: {z_score:.2f})")
        
        return anomalies
    
    def __del__(self):
        """Clean up database connections."""
        if hasattr(self, 'cursor'):
            self.cursor.close()
        if hasattr(self, 'conn'):
            self.conn.close()

if __name__ == "__main__":
    # Example usage
    db = DatabaseAnalytics("postgresql://user:pass@localhost/analytics")
    
    # Get Q3 2024 revenue
    q3_data = db.get_quarterly_revenue(2024, 3)
    print(f"Q3 2024 Total Revenue: ${q3_data['revenue'].sum():,.2f}")
    
    # Check for anomalies
    anomalies = db.detect_anomalies('revenue', threshold=3.0)