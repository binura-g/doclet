import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { EditorContent, useEditor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'
import Underline from '@tiptap/extension-underline'
import LinkExtension from '@tiptap/extension-link'
import * as Y from 'yjs'
import { getDocument, getCollabWsUrl, updateDocumentTitle } from '../api'
import { DocletProvider } from '../editor/DocletProvider'
import {
  base64ToBytes,
  colorFromSeed,
  getSessionClientId,
  getSessionDisplayName,
} from '../utils'

export default function EditorPage() {
  const { documentId } = useParams()
  const [displayName, setDisplayName] = useState('')
  const [isEditingTitle, setIsEditingTitle] = useState(false)
  const [titleError, setTitleError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [ready, setReady] = useState(false)
  const [provider, setProvider] = useState<DocletProvider | null>(null)
  const [userName] = useState(getSessionDisplayName())
  const [activeUsers, setActiveUsers] = useState<Array<{ id: number; name: string }>>([])

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
      name: userName,
      color: colorFromSeed(userName || clientId),
    }
    const nextProvider = new DocletProvider({
      documentId,
      clientId,
      wsUrl: getCollabWsUrl(),
      doc: ydoc,
      user,
    })
    setProvider(nextProvider)

    return () => {
      nextProvider.destroy()
      setProvider(null)
    }
  }, [ready, documentId, clientId, ydoc, userName])

  useEffect(() => {
    if (!provider) {
      return
    }
    provider.updateUser({
      name: userName,
      color: colorFromSeed(userName || clientId),
    })
  }, [provider, userName, clientId])

  useEffect(() => {
    if (!provider) {
      return
    }
    const updateUsers = () => {
      const users: Array<{ id: number; name: string }> = []
      const selfId = provider.awareness.clientID
      provider.awareness.getStates().forEach((state, id) => {
        if (id === selfId) {
          return
        }
        const name = state?.user?.name || 'Anonymous'
        users.push({ id, name })
      })
      setActiveUsers(users)
    }
    updateUsers()
    provider.awareness.on('update', updateUsers)
    return () => {
      provider.awareness.off('update', updateUsers)
    }
  }, [provider])

  const editor = useEditor(
    provider
      ? {
        extensions: [
          StarterKit.configure({ history: false }),
          Underline,
          LinkExtension.configure({ openOnClick: false }),
          Collaboration.configure({ document: ydoc }),
          CollaborationCursor.configure({
            provider,
            render: (user) => {
              const cursor = document.createElement('span')
              cursor.classList.add('doclet-cursor')
              const displayName = typeof user.name === 'string' ? user.name : ''
              const fallbackId = typeof user.id === 'string' ? user.id : 'user'
              const color = user.color || colorFromSeed(displayName || fallbackId)
              cursor.style.borderColor = color

              const label = document.createElement('span')
              label.classList.add('doclet-cursor-label')
              label.style.backgroundColor = color
              label.textContent = displayName || 'Anonymous'

              cursor.appendChild(label)
              return cursor
            },
          }),
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
      <div className="min-h-screen bg-zinc-950 text-white">
        <div className="mx-auto max-w-4xl px-6 py-12">
          <div className="doclet-card p-6">Missing document id.</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen bg-zinc-950 text-white">
        <div className="mx-auto max-w-4xl px-6 py-12">
          <div className="doclet-card p-6">{error}</div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-white via-zinc-50 to-emerald-50/60 text-zinc-900">
      <div className="mx-auto flex max-w-6xl flex-col gap-6 px-6 pb-16 pt-8">
        <div className="flex items-start justify-between gap-6">
          <div className="flex items-start gap-6">
            <Link
              className="doclet-button-secondary"
              to="/"
              aria-label="Back to home"
            >
              ← Back
            </Link>
            <div>
              {isEditingTitle ? (
                <input
                  className="doclet-input text-2xl font-semibold"
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                  onBlur={async () => {
                    setIsEditingTitle(false)
                    if (!documentId || displayName.trim() === '') {
                      return
                    }
                    try {
                      await updateDocumentTitle(documentId, displayName.trim())
                      setTitleError(null)
                    } catch (err) {
                      setTitleError((err as Error).message)
                    }
                  }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      setIsEditingTitle(false)
                      event.currentTarget.blur()
                    }
                  }}
                  autoFocus
                />
              ) : (
                <button
                  className="text-left text-3xl font-semibold text-zinc-900 hover:text-emerald-600"
                  onClick={() => setIsEditingTitle(true)}
                  type="button"
                >
                  {displayName || 'Untitled'}
                </button>
              )}
            </div>
          </div>
          <div>
            <div className="flex items-center gap-2 text-sm text-zinc-500">
              <span className="doclet-pill">You: {userName || 'Anonymous'}</span>
            </div>
            <div className="mt-4 flex flex-wrap items-center gap-2">
              {activeUsers.length === 0 ? (
                <span className="text-sm text-zinc-500">No active collaborators</span>
              ) : (
                <>
                  <span className="text-sm text-zinc-500">Active:</span>
                  {activeUsers.map((user) => (
                    <span key={user.id} className="doclet-pill">
                      {user.name}
                    </span>
                  ))}
                </>
              )}
            </div>
          </div>
        </div>
        {titleError ? <div className="text-sm text-rose-500">{titleError}</div> : null}

        <div className="doclet-card p-6">
          <div className="flex flex-wrap gap-2">
            <button
              className="doclet-toolbar-button"
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => editor?.chain().focus().toggleBold().run()}
            >
              Bold
            </button>
            <button
              className="doclet-toolbar-button"
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => editor?.chain().focus().toggleItalic().run()}
            >
              Italic
            </button>
            <button
              className="doclet-toolbar-button"
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => editor?.chain().focus().toggleUnderline().run()}
            >
              Underline
            </button>
            <button
              className="doclet-toolbar-button"
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => editor?.chain().focus().toggleHeading({ level: 2 }).run()}
            >
              Heading
            </button>
            <button
              className="doclet-toolbar-button"
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => editor?.chain().focus().toggleBulletList().run()}
            >
              Bullets
            </button>
            <button
              className="doclet-toolbar-button"
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => editor?.chain().focus().toggleOrderedList().run()}
            >
              Numbered
            </button>
            <button
              className="doclet-toolbar-button"
              type="button"
              onMouseDown={(event) => event.preventDefault()}
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
          <div
            className="mt-4 min-h-[460px] rounded-2xl border border-zinc-200 bg-white p-4"
            onClick={() => editor?.commands.focus()}
          >
            {editor ? <EditorContent editor={editor} /> : <div className="text-sm text-zinc-500">Loading editor…</div>}
          </div>
        </div>
      </div>
    </div>
  )
}
