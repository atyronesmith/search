#!/usr/bin/env python3
"""
Unstructured API Service
Provides document extraction with element-level metadata
"""

import os
import json
import logging
from typing import List, Dict, Any, Optional
from pathlib import Path
import tempfile

from flask import Flask, request, jsonify
from werkzeug.utils import secure_filename
from unstructured.partition.auto import partition
from unstructured.documents.elements import (
    Title, NarrativeText, ListItem, Table, 
    Header, Footer, PageBreak, Image, FigureCaption
)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)
app.config['MAX_CONTENT_LENGTH'] = 100 * 1024 * 1024  # 100MB max file size

# Supported file extensions
SUPPORTED_EXTENSIONS = {
    'pdf', 'docx', 'doc', 'pptx', 'ppt', 'xlsx', 'xls',
    'txt', 'md', 'rtf', 'html', 'htm', 'xml', 'csv', 'tsv',
    'epub', 'eml', 'msg', 'odt', 'png', 'jpg', 'jpeg', 'tiff', 'bmp'
}

def allowed_file(filename: str) -> bool:
    """Check if file extension is supported"""
    return '.' in filename and \
           filename.rsplit('.', 1)[1].lower() in SUPPORTED_EXTENSIONS

def element_to_dict(element) -> Dict[str, Any]:
    """Convert an Unstructured element to a dictionary"""
    elem_dict = {
        'type': element.__class__.__name__,
        'text': str(element),
        'metadata': {}
    }
    
    # Add metadata if available
    if hasattr(element, 'metadata'):
        metadata = element.metadata
        if metadata:
            elem_dict['metadata'] = metadata.to_dict() if hasattr(metadata, 'to_dict') else {}
            
            # Extract specific metadata fields
            if hasattr(metadata, 'page_number'):
                elem_dict['page_number'] = metadata.page_number
            if hasattr(metadata, 'filename'):
                elem_dict['filename'] = metadata.filename
            if hasattr(metadata, 'coordinates'):
                elem_dict['coordinates'] = metadata.coordinates
            if hasattr(metadata, 'parent_id'):
                elem_dict['parent_id'] = metadata.parent_id
                
    # Calculate category depth based on element type
    elem_dict['category_depth'] = calculate_category_depth(element)
    
    # Add element ID if available
    if hasattr(element, 'id'):
        elem_dict['element_id'] = element.id
    elif hasattr(element, '_element_id'):
        elem_dict['element_id'] = element._element_id
        
    return elem_dict

def calculate_category_depth(element) -> int:
    """Calculate hierarchy depth based on element type"""
    if isinstance(element, Title):
        # Try to detect title level from styling or content
        return 0  # Top-level by default
    elif isinstance(element, Header):
        return 1  # Section level
    elif isinstance(element, (NarrativeText, ListItem, Table)):
        return 2  # Content level
    elif isinstance(element, (Footer, PageBreak, FigureCaption)):
        return 3  # Auxiliary level
    return 2  # Default to content level

def extract_with_elements(file_path: str, strategy: str = 'hi_res') -> Dict[str, Any]:
    """Extract content from file with element-level metadata"""
    try:
        # Partition the document
        logger.info(f"Partitioning file: {file_path} with strategy: {strategy}")
        elements = partition(
            filename=file_path,
            strategy=strategy,
            include_metadata=True,
            infer_table_structure=True,
            extract_images_in_pdf=False,  # Disable to avoid issues
            extract_image_block_types=["Image", "Table"],
        )
        
        # Convert elements to dictionaries
        element_dicts = []
        full_text_parts = []
        
        for i, element in enumerate(elements):
            elem_dict = element_to_dict(element)
            elem_dict['index'] = i
            
            # Calculate character positions
            current_pos = sum(len(part) + 2 for part in full_text_parts)  # +2 for "\n\n"
            elem_dict['start'] = current_pos
            elem_dict['end'] = current_pos + len(str(element))
            
            element_dicts.append(elem_dict)
            full_text_parts.append(str(element))
        
        # Join all text
        full_text = "\n\n".join(full_text_parts)
        
        # Collect metadata
        metadata = {
            'elements': element_dicts,
            'element_count': len(elements),
            'extraction_method': 'unstructured',
            'strategy': strategy,
        }
        
        # Add document-level metadata if available
        if elements and hasattr(elements[0], 'metadata'):
            first_meta = elements[0].metadata
            if hasattr(first_meta, 'languages'):
                metadata['languages'] = first_meta.languages
            if hasattr(first_meta, 'filetype'):
                metadata['filetype'] = first_meta.filetype
                
        logger.info(f"Extracted {len(elements)} elements from {file_path}")
        
        return {
            'success': True,
            'content': full_text,
            'metadata': metadata
        }
        
    except Exception as e:
        logger.error(f"Error extracting from {file_path}: {str(e)}")
        return {
            'success': False,
            'content': '',
            'metadata': {},
            'error': str(e)
        }

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({'status': 'healthy', 'service': 'unstructured-api'})

@app.route('/extract', methods=['POST'])
def extract():
    """Extract content from uploaded file"""
    try:
        # Check if file is in request
        if 'file' not in request.files:
            return jsonify({
                'success': False,
                'error': 'No file provided'
            }), 400
            
        file = request.files['file']
        
        if file.filename == '':
            return jsonify({
                'success': False,
                'error': 'Empty filename'
            }), 400
            
        if not allowed_file(file.filename):
            return jsonify({
                'success': False,
                'error': f'Unsupported file type: {file.filename}'
            }), 400
        
        # Get extraction parameters
        strategy = request.form.get('strategy', 'hi_res')
        include_metadata = request.form.get('include_metadata', 'true').lower() == 'true'
        
        # Save file temporarily
        with tempfile.NamedTemporaryFile(delete=False, suffix=Path(file.filename).suffix) as tmp_file:
            file.save(tmp_file.name)
            tmp_path = tmp_file.name
            
        try:
            # Extract content with elements
            result = extract_with_elements(tmp_path, strategy)
            
            if not include_metadata:
                # Remove detailed metadata if not requested
                result['metadata'] = {
                    'element_count': result['metadata'].get('element_count', 0)
                }
                
            return jsonify(result)
            
        finally:
            # Clean up temp file
            try:
                os.unlink(tmp_path)
            except:
                pass
                
    except Exception as e:
        logger.error(f"Error in extract endpoint: {str(e)}")
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

@app.route('/supported_types', methods=['GET'])
def supported_types():
    """Return list of supported file types"""
    return jsonify({
        'extensions': list(SUPPORTED_EXTENSIONS),
        'categories': {
            'documents': ['pdf', 'docx', 'doc', 'pptx', 'ppt', 'rtf', 'odt'],
            'spreadsheets': ['xlsx', 'xls', 'csv', 'tsv'],
            'text': ['txt', 'md', 'html', 'htm', 'xml'],
            'email': ['eml', 'msg'],
            'images': ['png', 'jpg', 'jpeg', 'tiff', 'bmp'],
            'ebooks': ['epub']
        }
    })

if __name__ == '__main__':
    port = int(os.environ.get('UNSTRUCTURED_PORT', 8001))
    host = os.environ.get('UNSTRUCTURED_HOST', '0.0.0.0')
    debug = os.environ.get('UNSTRUCTURED_DEBUG', 'false').lower() == 'true'
    
    logger.info(f"Starting Unstructured API on {host}:{port}")
    app.run(host=host, port=port, debug=debug)