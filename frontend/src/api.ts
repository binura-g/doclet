const docServiceUrl = import.meta.env.VITE_DOC_SERVICE_URL || 'http://localhost:8080'

export type DocumentListItem = {
  document_id: string
  displayName: string
  updated_at: string
}

export type DocumentResponse = {
  document_id: string
  displayName: string
  content: string
  created_at: string
  updated_at: string
}

export async function listDocuments(query: string): Promise<DocumentListItem[]> {
  const url = new URL(`${docServiceUrl}/documents`)
  if (query) {
    url.searchParams.set('query', query)
  }
  const res = await fetch(url.toString())
  if (!res.ok) {
    throw new Error('Failed to load documents')
  }
  const data = await res.json()
  return data.items || []
}

export async function createDocument(displayName: string): Promise<DocumentResponse> {
  const res = await fetch(`${docServiceUrl}/documents`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ displayName }),
  })
  if (!res.ok) {
    throw new Error('Failed to create document')
  }
  return res.json()
}

export async function getDocument(documentId: string): Promise<DocumentResponse> {
  const res = await fetch(`${docServiceUrl}/documents/${documentId}`)
  if (!res.ok) {
    throw new Error('Document not found')
  }
  return res.json()
}

export function getCollabWsUrl(): string {
  return import.meta.env.VITE_COLLAB_WS_URL || 'ws://localhost:8090/ws'
}
