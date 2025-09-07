# Test Data Suite for File Search System

This directory contains a comprehensive test suite of files designed to test various search capabilities of the file search system.

## Directory Structure

### `/emotional`
Contains text files with various emotional content:
- **sad_memoir.txt** - Text with sad, melancholic content (grief, loss, depression)
- **happy_celebration.txt** - Joyful, happy content (celebration, joy, happiness)
- **angry_letter.md** - Angry, frustrated content (rage, fury, indignation)

### `/financial`
Financial documents and data:
- **q3_financial_report.csv** - Q3 2024 financial data with revenue, expenses
- **budget_2024.csv** - Annual budget tracking with variances

### `/medical`
Medical and healthcare documents:
- **patient_report.txt** - Medical report with diagnoses, medications, lab results

### `/technical`
Technical documentation and code:
- **kubernetes_deployment.yaml** - K8s deployment configuration
- **machine_learning_guide.md** - ML model documentation with metrics
- **api_response.json** - Sample API JSON response
- **database_query.py** - Python code for SQL analytics
- **vector_search.go** - Go code for vector similarity search

### `/literature`
Literary content:
- **shakespeare_sonnet.txt** - Classic literature with analysis

### Root Level
- **personal_info.txt** - Contains SSN, credit card (test data for pattern detection)

## Test Queries

You can test the following types of queries:

### Emotional Searches
- "Find files that are sad"
- "Happy documents"
- "Angry or frustrated content"

### Financial Searches
- "Q3 financial reports"
- "Budget variance"
- "Revenue and expenses"

### Medical Searches
- "Patient diagnosis"
- "Medications and prescriptions"
- "Blood pressure results"

### Technical Searches
- "Kubernetes deployment"
- "Machine learning accuracy"
- "SQL queries"
- "Vector embeddings"

### Pattern Detection
- "Find files with social security numbers"
- "Credit card numbers"
- "Email addresses"

### Temporal Searches
- "Documents from Q3 2024"
- "Files created in July"
- "Recent financial reports"

## Notes
- All personal information is fictional and for testing purposes only
- Files are kept small for efficient testing
- Content covers diverse domains to test semantic understanding