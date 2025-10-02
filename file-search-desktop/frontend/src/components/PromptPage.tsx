import { useState, useEffect } from 'react'
import './PromptPage.css'

export default function PromptPage() {
  const [prompt, setPrompt] = useState('')
  const [originalPrompt, setOriginalPrompt] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error', text: string } | null>(null)
  const [hasChanges, setHasChanges] = useState(false)

  const fetchPrompt = async () => {
    setLoading(true)
    setMessage(null)
    try {
      const currentPrompt = await window.go.main.App.GetPrompt()
      setPrompt(currentPrompt)
      setOriginalPrompt(currentPrompt)
      setHasChanges(false)
    } catch (err) {
      setMessage({
        type: 'error',
        text: err instanceof Error ? err.message : 'Failed to fetch prompt'
      })
    } finally {
      setLoading(false)
    }
  }

  const savePrompt = async () => {
    setSaving(true)
    setMessage(null)
    try {
      const response = await window.go.main.App.UpdatePrompt(prompt)
      setMessage({
        type: 'success',
        text: response || 'Prompt updated successfully'
      })
      setOriginalPrompt(prompt)
      setHasChanges(false)
      
      // Clear success message after 3 seconds
      setTimeout(() => {
        setMessage(null)
      }, 3000)
    } catch (err) {
      setMessage({
        type: 'error',
        text: err instanceof Error ? err.message : 'Failed to save prompt'
      })
    } finally {
      setSaving(false)
    }
  }

  const resetPrompt = () => {
    setPrompt(originalPrompt)
    setHasChanges(false)
    setMessage(null)
  }

  const handlePromptChange = (value: string) => {
    setPrompt(value)
    setHasChanges(value !== originalPrompt)
  }

  useEffect(() => {
    fetchPrompt()
  }, [])

  const insertVariable = (variable: string) => {
    const textarea = document.getElementById('prompt-textarea') as HTMLTextAreaElement
    if (textarea) {
      const start = textarea.selectionStart
      const end = textarea.selectionEnd
      const newValue = prompt.substring(0, start) + variable + prompt.substring(end)
      handlePromptChange(newValue)
      
      // Set cursor position after inserted text
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = start + variable.length
        textarea.focus()
      }, 0)
    }
  }

  const promptVariables = [
    { name: '{{QUERY}}', description: 'The user\'s search query' },
    { name: '{{CONTEXT}}', description: 'Retrieved document context' },
    { name: '{{DATE}}', description: 'Current date and time' },
    { name: '{{FILE_TYPES}}', description: 'List of available file types' },
    { name: '{{TEMPORAL_CONTEXT}}', description: 'Temporal query context' }
  ]

  return (
    <div className="prompt-page prompt-page-wrapper">
      <div className="prompt-header">
        <h2>LLM Prompt Template</h2>
        <div className="prompt-actions">
          {hasChanges && (
            <span className="unsaved-indicator">Unsaved changes</span>
          )}
          <div className="button-group">
            <button 
              onClick={resetPrompt} 
              disabled={!hasChanges || loading || saving}
              style={{
                backgroundColor: (!hasChanges || loading || saving) ? '#93bbde' : '#3182ce',
                color: 'white',
                padding: '4px 12px',
                border: 'none',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: '500',
                cursor: (!hasChanges || loading || saving) ? 'not-allowed' : 'pointer',
                minWidth: '100px',
                opacity: (!hasChanges || loading || saving) ? '0.7' : '1'
              }}
            >
              Reset
            </button>
            <button 
              onClick={savePrompt} 
              disabled={!hasChanges || loading || saving}
              style={{
                backgroundColor: (!hasChanges || loading || saving) ? '#93bbde' : '#3182ce',
                color: 'white',
                padding: '4px 12px',
                border: 'none',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: '500',
                cursor: (!hasChanges || loading || saving) ? 'not-allowed' : 'pointer',
                minWidth: '100px',
                opacity: (!hasChanges || loading || saving) ? '0.7' : '1'
              }}
            >
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>
      </div>

      {message && (
        <div className={`message ${message.type}`}>
          {message.text}
        </div>
      )}

      <div className="prompt-content">
        <div className="prompt-editor">
          <div className="editor-header">
            <h3>Prompt Template</h3>
            <div className="character-count">
              {prompt.length} characters
            </div>
          </div>
          
          {loading ? (
            <div className="loading-state">Loading prompt template...</div>
          ) : (
            <textarea
              id="prompt-textarea"
              className="prompt-textarea"
              value={prompt}
              onChange={(e) => handlePromptChange(e.target.value)}
              placeholder="Enter your LLM prompt template here..."
              spellCheck={false}
            />
          )}
        </div>

        <div className="prompt-sidebar">
          <div className="variables-section">
            <h3>Available Variables</h3>
            <p className="variables-description">
              Click to insert variables into the prompt template
            </p>
            <div className="variables-list">
              {promptVariables.map((variable) => (
                <div 
                  key={variable.name}
                  className="variable-item"
                  onClick={() => insertVariable(variable.name)}
                >
                  <div className="variable-name">{variable.name}</div>
                  <div className="variable-description">{variable.description}</div>
                </div>
              ))}
            </div>
          </div>

          <div className="tips-section">
            <h3>Tips</h3>
            <ul className="tips-list">
              <li>Use variables to make your prompt dynamic</li>
              <li>Keep instructions clear and concise</li>
              <li>Test your prompt with different query types</li>
              <li>Include examples for better results</li>
              <li>Specify the expected output format</li>
            </ul>
          </div>

          <div className="info-section">
            <h3>How It Works</h3>
            <p>
              This prompt template is used to enhance search queries with the LLM model. 
              When a user performs a search that requires natural language understanding, 
              the query is processed using this template to generate better search results.
            </p>
            <p>
              The template should guide the LLM to understand user intent and generate 
              appropriate search parameters like temporal filters, content patterns, 
              and semantic queries.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}