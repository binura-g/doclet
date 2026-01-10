import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createDocument, listDocuments, DocumentListItem } from '../api'

export default function HomePage() {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [docs, setDocs] = useState<DocumentListItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [displayName, setDisplayName] = useState('')

  const searchLabel = useMemo(() => (query ? `Results for "${query}"` : 'Recent documents'), [query])

  const refresh = async (nextQuery: string) => {
    setLoading(true)
    setError(null)
    try {
      const items = await listDocuments(nextQuery)
      setDocs(items)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refresh('')
  }, [])

  const onSearch = (event: React.FormEvent) => {
    event.preventDefault()
    refresh(query)
  }

  const onCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    setLoading(true)
    setError(null)
    try {
      const doc = await createDocument(displayName)
      navigate(`/doc/${doc.document_id}`)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="container">
      <div className="header">
        <div>
          <h1>Doclet</h1>
          <p className="meta">Fast, anonymous collaboration on rich-text documents.</p>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 20 }}>
        <form onSubmit={onCreate} style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <input
            className="input"
            style={{ flex: 1, minWidth: 200 }}
            placeholder="Document name (optional)"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
          />
          <button className="button" type="submit">Create document</button>
        </form>
      </div>

      <div className="card" style={{ marginBottom: 20 }}>
        <form onSubmit={onSearch} style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <input
            className="input"
            placeholder="Search by document name"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <button className="button secondary" type="submit">Search</button>
        </form>
      </div>

      <div className="card">
        <div className="list">
          <div className="list-item">
            <strong>{searchLabel}</strong>
            {loading ? <span className="meta">Loading…</span> : null}
          </div>
          {error ? <div className="meta">{error}</div> : null}
          {docs.length === 0 && !loading ? <div className="meta">No documents found.</div> : null}
          {docs.map((doc) => (
            <div className="list-item" key={doc.document_id}>
              <div>
                <div>{doc.displayName || 'Untitled'}</div>
                <div className="meta">Updated {new Date(doc.updated_at).toLocaleString()}</div>
              </div>
              <button className="button" onClick={() => navigate(`/doc/${doc.document_id}`)}>Open</button>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
