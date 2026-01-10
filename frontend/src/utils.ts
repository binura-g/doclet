export function base64ToBytes(input: string): Uint8Array {
  if (!input) {
    return new Uint8Array()
  }
  const binary = atob(input)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte)
  })
  return btoa(binary)
}

export function getSessionClientId(): string {
  const key = 'doclet_client_id'
  const existing = sessionStorage.getItem(key)
  if (existing) {
    return existing
  }
  const id = crypto.randomUUID()
  sessionStorage.setItem(key, id)
  return id
}

export function colorFromClientId(clientId: string): string {
  let hash = 0
  for (let i = 0; i < clientId.length; i += 1) {
    hash = clientId.charCodeAt(i) + ((hash << 5) - hash)
  }
  const hue = Math.abs(hash) % 360
  return `hsl(${hue}, 70%, 45%)`
}
