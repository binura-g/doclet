import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { EditorContent, useEditor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'
import Underline from '@tiptap/extension-underline'
import LinkExtension from '@tiptap/extension-link'
import * as Y from 'yjs'
import { getDocument, getCollabWsUrl } from '../api'
import { DocletProvider } from '../editor/DocletProvider'
import { base64ToBytes, colorFromClientId, getSessionClientId } from '../utils'

export default function EditorPage() {
  const { documentId } = useParams()
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState<'connected' | 'disconnected'>('disconnected')
  const [ready, setReady] = useState(false)
  const [provider, setProvider] = useState<DocletProvider | null>(null)

  const clientId = useMemo(() => getSessionClientId(), [])
  const ydoc = useMemo(() => new Y.Doc(), [documentId])

  useEffect(() => {
    if (!documentId) {
      return
    }
    let isMounted = true
    const loadDoc = async () => {
      try {
        const doc = await getDocument(documentId)
        if (!isMounted) {
          return
        }
        setDisplayName(doc.displayName)
        const update = base64ToBytes(doc.content)
        if (update.length > 0) {
          Y.applyUpdate(ydoc, update)
        }
        setReady(true)
      } catch (err) {
        setError((err as Error).message)
      }
    }
    loadDoc()

    return () => {
      isMounted = false
    }
  }, [documentId, ydoc])

  useEffect(() => {
    if (!ready || !documentId) {
      return
    }
    const user = {
      name: `User ${clientId.slice(0, 6)}`,
      color: colorFromClientId(clientId),
    }
    const nextProvider = new DocletProvider({
      documentId,
      clientId,
      wsUrl: getCollabWsUrl(),
      doc: ydoc,
      user,
      onStatus: setStatus,
    })
    setProvider(nextProvider)

    return () => {
      nextProvider.destroy()
      setProvider(null)
      setStatus('disconnected')
    }
  }, [ready, documentId, clientId, ydoc])

  const editor = useEditor(
    provider
      ? {
          extensions: [
            StarterKit.configure({ history: false }),
            Underline,
            LinkExtension.configure({ openOnClick: false }),
            Collaboration.configure({ document: ydoc }),
            CollaborationCursor.configure({ provider }),
          ],
          editorProps: {
            attributes: {
              class: 'doclet-editor',
            },
          },
        }
      : {
          extensions: [StarterKit.configure({ history: false }), Underline, LinkExtension],
          editable: false,
        },
    [provider]
  )

  if (!documentId) {
    return (
      <div className="container">
        <div className="card">Missing document id.</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="container">
        <div className="card">{error}</div>
      </div>
    )
  }

  return (
    <div className="container">
      <div className="header">
        <div>
          <h2>{displayName || 'Untitled'}</h2>
          <div className="meta">Document ID: {documentId}</div>
        </div>
        <Link className="button secondary" to="/">Home</Link>
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <div className="toolbar">
          <button onClick={() => editor?.chain().focus().toggleBold().run()}>Bold</button>
          <button onClick={() => editor?.chain().focus().toggleItalic().run()}>Italic</button>
          <button onClick={() => editor?.chain().focus().toggleUnderline().run()}>Underline</button>
          <button onClick={() => editor?.chain().focus().toggleHeading({ level: 2 }).run()}>Heading</button>
          <button onClick={() => editor?.chain().focus().toggleBulletList().run()}>Bullets</button>
          <button onClick={() => editor?.chain().focus().toggleOrderedList().run()}>Numbered</button>
          <button
            onClick={() => {
              const url = prompt('Enter URL')
              if (url) {
                editor?.chain().focus().setLink({ href: url }).run()
              }
            }}
          >
            Link
          </button>
        </div>
        <div className="editor-shell">
          {editor ? <EditorContent editor={editor} /> : <div>Loading editor…</div>}
        </div>
        <div className="status">Status: {status}</div>
      </div>
    </div>
  )
}
