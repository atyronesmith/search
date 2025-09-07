"""
FastAPI application for document processing service
"""

import os
import tempfile
import logging
from datetime import datetime
from pathlib import Path
from typing import Optional, Dict, Any

from fastapi import FastAPI, UploadFile, File, HTTPException, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from models import (
    ExtractionResult, ProcessingRequest, HealthCheck, 
    ServiceInfo, DocumentElement, ElementType
)
from extractor import DocumentExtractorManager

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# FastAPI app initialization
app = FastAPI(
    title="Document Processing Service",
    description="Enhanced document processing with structured extraction",
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc"
)

# CORS middleware for cross-origin requests
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Configure appropriately for production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Initialize extractor manager
extractor_manager = DocumentExtractorManager()


@app.get("/", response_model=ServiceInfo)
async def root():
    """Service information endpoint"""
    return ServiceInfo()


@app.get("/health", response_model=HealthCheck)
async def health_check():
    """Health check endpoint"""
    
    # Check dependencies
    dependencies = {}
    
    try:
        import pypdfium2
        dependencies["pypdfium2"] = "available"
    except ImportError:
        dependencies["pypdfium2"] = "missing"
    
    try:
        from docx import Document
        dependencies["python-docx"] = "available"
    except ImportError:
        dependencies["python-docx"] = "missing"
    
    try:
        from pptx import Presentation
        dependencies["python-pptx"] = "available"
    except ImportError:
        dependencies["python-pptx"] = "missing"
    
    # Check pdftotext availability
    import subprocess
    try:
        result = subprocess.run(['which', 'pdftotext'], capture_output=True)
        dependencies["pdftotext"] = "available" if result.returncode == 0 else "missing"
    except Exception:
        dependencies["pdftotext"] = "error"
    
    return HealthCheck(
        status="healthy",
        version="1.0.0",
        timestamp=datetime.utcnow().isoformat(),
        dependencies=dependencies
    )


@app.post("/extract", response_model=ExtractionResult)
async def extract_document_upload(
    file: UploadFile = File(...),
    extraction_method: str = "auto",
    background_tasks: BackgroundTasks = BackgroundTasks()
):
    """Extract content from uploaded document"""
    
    if not file.filename:
        raise HTTPException(status_code=400, detail="No filename provided")
    
    # Check file extension
    file_ext = Path(file.filename).suffix.lower()
    supported_extensions = ['.pdf', '.docx', '.pptx']
    
    if file_ext not in supported_extensions:
        raise HTTPException(
            status_code=400, 
            detail=f"Unsupported file type: {file_ext}. Supported types: {supported_extensions}"
        )
    
    # Create temporary file
    temp_file = None
    try:
        # Save uploaded file to temporary location
        with tempfile.NamedTemporaryFile(delete=False, suffix=file_ext) as tmp:
            content = await file.read()
            tmp.write(content)
            temp_file = tmp.name
        
        logger.info(f"Processing uploaded file: {file.filename} ({len(content)} bytes)")
        
        # Extract document content
        result = extractor_manager.extract_document(
            file_path=temp_file,
            extraction_method=extraction_method
        )
        
        # Add file metadata
        result.metadata.update({
            'original_filename': file.filename,
            'file_size': len(content),
            'file_type': file.content_type or 'unknown'
        })
        
        # Schedule cleanup
        background_tasks.add_task(cleanup_temp_file, temp_file)
        
        return result
        
    except Exception as e:
        logger.error(f"Error processing upload: {str(e)}")
        
        # Cleanup on error
        if temp_file and os.path.exists(temp_file):
            try:
                os.unlink(temp_file)
            except Exception as cleanup_error:
                logger.warning(f"Failed to cleanup temp file: {cleanup_error}")
        
        raise HTTPException(status_code=500, detail=f"Processing error: {str(e)}")


@app.post("/extract/path", response_model=ExtractionResult)
async def extract_document_path(request: ProcessingRequest):
    """Extract content from document at specified path"""
    
    if not request.file_path:
        raise HTTPException(status_code=400, detail="file_path is required")
    
    file_path = Path(request.file_path)
    
    # Security check - ensure path exists and is readable
    if not file_path.exists():
        raise HTTPException(status_code=404, detail=f"File not found: {request.file_path}")
    
    if not file_path.is_file():
        raise HTTPException(status_code=400, detail=f"Path is not a file: {request.file_path}")
    
    # Check file extension
    file_ext = file_path.suffix.lower()
    supported_extensions = ['.pdf', '.docx', '.pptx']
    
    if file_ext not in supported_extensions:
        raise HTTPException(
            status_code=400, 
            detail=f"Unsupported file type: {file_ext}. Supported types: {supported_extensions}"
        )
    
    try:
        logger.info(f"Processing file at path: {request.file_path}")
        
        # Extract document content
        result = extractor_manager.extract_document(
            file_path=request.file_path,
            extraction_method=request.extraction_method or "auto",
            options=request.options
        )
        
        # Add file metadata
        result.metadata.update({
            'file_size': file_path.stat().st_size,
            'file_modified': datetime.fromtimestamp(file_path.stat().st_mtime).isoformat()
        })
        
        return result
        
    except Exception as e:
        logger.error(f"Error processing file path: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Processing error: {str(e)}")


@app.get("/extractors")
async def list_extractors():
    """List available extraction methods"""
    return {
        "available_extractors": list(extractor_manager.extractors.keys()),
        "supported_formats": {
            ".pdf": ["pdftotext", "pypdfium2"],
            ".docx": ["python-docx"],
            ".pptx": ["python-pptx"]
        }
    }


@app.post("/test/sample")
async def test_sample_extraction():
    """Test extraction with sample documents (for development)"""
    
    # This endpoint is for development/testing purposes
    sample_results = []
    
    # Look for sample files in Downloads
    downloads_path = Path.home() / "Downloads"
    sample_files = []
    
    for ext in ['.pdf', '.docx', '.pptx']:
        files = list(downloads_path.glob(f"*{ext}"))
        sample_files.extend(files[:2])  # Limit to 2 files per type
    
    if not sample_files:
        return {
            "message": "No sample files found in Downloads directory",
            "searched_extensions": [".pdf", ".docx", ".pptx"]
        }
    
    for file_path in sample_files[:5]:  # Limit total files
        try:
            result = extractor_manager.extract_document(str(file_path))
            sample_results.append({
                "file": str(file_path.name),
                "result": result.dict()
            })
        except Exception as e:
            sample_results.append({
                "file": str(file_path.name),
                "error": str(e)
            })
    
    return {
        "sample_count": len(sample_results),
        "results": sample_results
    }


def cleanup_temp_file(file_path: str):
    """Clean up temporary file"""
    try:
        if os.path.exists(file_path):
            os.unlink(file_path)
            logger.info(f"Cleaned up temp file: {file_path}")
    except Exception as e:
        logger.warning(f"Failed to cleanup temp file {file_path}: {e}")


# Error handlers
@app.exception_handler(Exception)
async def general_exception_handler(request, exc):
    """Handle general exceptions"""
    logger.error(f"Unhandled exception: {str(exc)}", exc_info=True)
    return JSONResponse(
        status_code=500,
        content={"detail": "Internal server error", "error": str(exc)}
    )


if __name__ == "__main__":
    import uvicorn
    
    # Configuration
    host = os.getenv("HOST", "127.0.0.1")
    port = int(os.getenv("PORT", "8081"))
    
    logger.info(f"Starting Document Processing Service on {host}:{port}")
    
    uvicorn.run(
        app,
        host=host,
        port=port,
        log_level="info",
        access_log=True
    )