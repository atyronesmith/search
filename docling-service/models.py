"""
Data models for the document processing service
"""

from pydantic import BaseModel, Field
from typing import List, Optional, Dict, Any
from enum import Enum


class ElementType(str, Enum):
    """Document element types"""
    HEADING = "heading"
    PARAGRAPH = "paragraph"
    TABLE = "table"
    FIGURE = "figure"
    LIST = "list"
    TITLE = "title"
    PAGE_TEXT = "page_text"
    SLIDE_TITLE = "slide_title"
    SLIDE_CONTENT = "slide_content"


class BoundingBox(BaseModel):
    """Bounding box coordinates"""
    x: float
    y: float
    width: float
    height: float


class DocumentElement(BaseModel):
    """Represents a structured document element"""
    element_type: ElementType = Field(..., description="Type of document element")
    content: str = Field(..., description="Text content of the element")
    page_number: int = Field(default=0, description="Page number (0-indexed)")
    structure_data: Optional[Dict[str, Any]] = Field(default=None, description="Additional structural metadata")
    bbox: Optional[BoundingBox] = Field(default=None, description="Bounding box coordinates")
    metadata: Optional[Dict[str, Any]] = Field(default_factory=dict, description="Additional metadata")


class ExtractionResult(BaseModel):
    """Result of document extraction"""
    success: bool = Field(..., description="Whether extraction was successful")
    elements: List[DocumentElement] = Field(default_factory=list, description="Extracted document elements")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Document-level metadata")
    processing_time: float = Field(..., description="Time taken for processing in seconds")
    error_message: Optional[str] = Field(default=None, description="Error message if extraction failed")
    extraction_method: str = Field(..., description="Method used for extraction")


class ProcessingRequest(BaseModel):
    """Request for document processing"""
    file_path: Optional[str] = Field(default=None, description="Path to file on server")
    extraction_method: Optional[str] = Field(default="auto", description="Preferred extraction method")
    options: Optional[Dict[str, Any]] = Field(default_factory=dict, description="Additional processing options")


class HealthCheck(BaseModel):
    """Health check response"""
    status: str = Field(..., description="Service status")
    version: str = Field(..., description="Service version")
    timestamp: str = Field(..., description="Current timestamp")
    dependencies: Dict[str, str] = Field(default_factory=dict, description="Dependency status")


class ServiceInfo(BaseModel):
    """Service information"""
    name: str = "Document Processing Service"
    version: str = "1.0.0"
    description: str = "FastAPI service for enhanced document processing"
    supported_formats: List[str] = Field(default_factory=lambda: [
        ".pdf", ".docx", ".pptx", ".doc", ".xls", ".xlsx", ".csv"
    ])
    extraction_methods: List[str] = Field(default_factory=lambda: [
        "pypdfium2", "python-docx", "python-pptx", "auto"
    ])