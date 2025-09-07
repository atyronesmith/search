"""
Standalone Docling Service with isolated dependencies
Runs in Docker container with Python 3.11 to avoid dependency conflicts
"""

from fastapi import FastAPI, UploadFile, File, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from typing import List, Optional, Dict, Any
import tempfile
import time
import os

# Import docling (will work in container with Python 3.11)
try:
    from docling.document_converter import DocumentConverter
    DOCLING_AVAILABLE = True
except ImportError as e:
    DOCLING_AVAILABLE = False
    import_error = str(e)

app = FastAPI(
    title="Docling Service",
    description="Isolated Docling document processing service",
    version="1.0.0"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

class DoclingElement(BaseModel):
    element_type: str
    content: str
    page_number: int = 0
    metadata: Optional[Dict[str, Any]] = {}
    bbox: Optional[Dict[str, float]] = None

class DoclingResult(BaseModel):
    success: bool
    elements: List[DoclingElement]
    metadata: Dict[str, Any]
    processing_time: float
    error_message: Optional[str] = None
    extraction_method: str = "docling"

@app.get("/")
async def root():
    return {
        "service": "Docling Document Processor",
        "version": "1.0.0",
        "docling_available": DOCLING_AVAILABLE,
        "supported_formats": [".pdf", ".docx", ".pptx", ".xlsx", ".html", ".md"]
    }

@app.get("/health")
async def health_check():
    message = "OK"
    if not DOCLING_AVAILABLE:
        message = f"Docling not available: {import_error if 'import_error' in globals() else 'Import failed'}"
    
    return {
        "status": "healthy" if DOCLING_AVAILABLE else "degraded",
        "docling_available": DOCLING_AVAILABLE,
        "message": message
    }

@app.post("/extract", response_model=DoclingResult)
async def extract_document(file: UploadFile = File(...)):
    """Extract structured content from document using Docling"""
    
    if not DOCLING_AVAILABLE:
        raise HTTPException(
            status_code=503, 
            detail="Docling is not available. This service requires Python 3.11 or earlier."
        )
    
    start_time = time.time()
    temp_file = None
    
    try:
        # Save uploaded file
        with tempfile.NamedTemporaryFile(delete=False, suffix=file.filename) as tmp:
            content = await file.read()
            tmp.write(content)
            temp_file = tmp.name
        
        # Process with Docling
        converter = DocumentConverter()
        result = converter.convert(temp_file)
        
        # Extract structured elements
        elements = []
        if result and result.document:
            doc = result.document
            
            # Get markdown export for rich content
            markdown_content = doc.export_to_markdown()
            
            # Create elements from document structure
            # Add main document content as markdown
            elements.append(DoclingElement(
                element_type="markdown",
                content=markdown_content,
                page_number=0,
                metadata={
                    "extraction_method": "docling",
                    "export_format": "markdown"
                }
            ))
            
            # Extract text elements if available
            if hasattr(doc, 'texts') and doc.texts:
                for idx, text_item in enumerate(doc.texts):
                    elements.append(DoclingElement(
                        element_type="text",
                        content=str(text_item),
                        page_number=0,
                        metadata={
                            "extraction_method": "docling",
                            "text_index": idx
                        }
                    ))
            
            # Extract table data if available  
            if hasattr(doc, 'tables') and doc.tables:
                for idx, table in enumerate(doc.tables):
                    elements.append(DoclingElement(
                        element_type="table",
                        content=str(table),
                        page_number=0,
                        metadata={
                            "extraction_method": "docling",
                            "table_index": idx
                        }
                    ))
            
            # Get document metadata
            doc_metadata = {
                "filename": file.filename,
                "total_pages": doc.page_count if hasattr(doc, 'page_count') else 0,
                "element_count": len(elements),
                "format": doc.format if hasattr(doc, 'format') else "unknown"
            }
            
            processing_time = time.time() - start_time
            
            return DoclingResult(
                success=True,
                elements=elements,
                metadata=doc_metadata,
                processing_time=processing_time
            )
        else:
            raise Exception("Docling conversion returned empty result")
            
    except Exception as e:
        return DoclingResult(
            success=False,
            elements=[],
            metadata={"filename": file.filename},
            processing_time=time.time() - start_time,
            error_message=str(e)
        )
    finally:
        # Cleanup
        if temp_file and os.path.exists(temp_file):
            os.unlink(temp_file)

@app.post("/extract/advanced", response_model=DoclingResult)
async def extract_document_advanced(
    file: UploadFile = File(...),
    export_format: str = "markdown",
    ocr_enabled: bool = False,
    table_extraction: bool = True
):
    """Advanced extraction with configurable options"""
    
    if not DOCLING_AVAILABLE:
        raise HTTPException(
            status_code=503,
            detail="Docling is not available. This service requires Python 3.11 or earlier."
        )
    
    start_time = time.time()
    temp_file = None
    
    try:
        # Save uploaded file
        with tempfile.NamedTemporaryFile(delete=False, suffix=file.filename) as tmp:
            content = await file.read()
            tmp.write(content)
            temp_file = tmp.name
        
        # Configure Docling converter
        converter_config = {
            "ocr_enabled": ocr_enabled,
            "table_extraction": table_extraction,
        }
        
        converter = DocumentConverter(**converter_config)
        result = converter.convert(temp_file)
        
        # Export in requested format
        if export_format == "markdown" and hasattr(result.document, 'export_to_markdown'):
            markdown_content = result.document.export_to_markdown()
            elements = [DoclingElement(
                element_type="markdown",
                content=markdown_content,
                page_number=0,
                metadata={"format": "markdown"}
            )]
        elif export_format == "json" and hasattr(result.document, 'export_to_json'):
            json_content = result.document.export_to_json()
            elements = [DoclingElement(
                element_type="json",
                content=json_content,
                page_number=0,
                metadata={"format": "json"}
            )]
        else:
            # Default structured extraction
            elements = []
            for element in result.document.elements:
                elements.append(DoclingElement(
                    element_type=str(element.element_type),
                    content=str(element),
                    page_number=getattr(element, 'page_number', 0)
                ))
        
        processing_time = time.time() - start_time
        
        return DoclingResult(
            success=True,
            elements=elements,
            metadata={
                "filename": file.filename,
                "export_format": export_format,
                "ocr_enabled": ocr_enabled,
                "table_extraction": table_extraction
            },
            processing_time=processing_time
        )
        
    except Exception as e:
        return DoclingResult(
            success=False,
            elements=[],
            metadata={"filename": file.filename},
            processing_time=time.time() - start_time,
            error_message=str(e)
        )
    finally:
        if temp_file and os.path.exists(temp_file):
            os.unlink(temp_file)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8082)