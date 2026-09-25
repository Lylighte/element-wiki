// Test-only OIDC provider for browser acceptance. Never imported by the wiki server.
import { createHmac, createHash, randomBytes } from 'node:crypto'
import { createServer } from 'node:http'

const port = Number(process.env.TEST_IDP_PORT || 18081)
const issuer = `http://127.0.0.1:${port}`
const clientID = 'element-wiki-e2e'
const clientSecret = 'element-wiki-e2e-only-secret'
const callback = 'http://127.0.0.1:18080/v1/auth/oidc/callback'
const codes = new Map()
const encoded = (data) => Buffer.from(JSON.stringify(data)).toString('base64url')

function sendJSON(res, status, data) {
  res.writeHead(status, { 'content-type': 'application/json' })
  res.end(JSON.stringify(data))
}

function tokenFor(code) {
  const now = Math.floor(Date.now() / 1000)
  const header = encoded({ alg: 'HS256', typ: 'JWT' })
  const claims = encoded({
    iss: issuer,
    aud: clientID,
    sub: code.identity,
    email: `${code.identity}@e2e.local`,
    name: code.identity,
    nonce: code.nonce,
    iat: now,
    exp: now + 300,
  })
  const body = `${header}.${claims}`
  const signature = createHmac('sha256', clientSecret).update(body).digest('base64url')
  return `${body}.${signature}`
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url, issuer)
  if (url.pathname === '/healthz') return sendJSON(res, 200, { ok: true })
  if (url.pathname === '/.well-known/openid-configuration') {
    return sendJSON(res, 200, {
      issuer,
      authorization_endpoint: `${issuer}/authorize`,
      token_endpoint: `${issuer}/token`,
    })
  }
  if (url.pathname === '/authorize') {
    if (url.searchParams.get('client_id') !== clientID ||
        url.searchParams.get('redirect_uri') !== callback ||
        url.searchParams.get('code_challenge_method') !== 'S256') {
      return sendJSON(res, 400, { error: 'invalid_request' })
    }
    const grant = new URL(`${issuer}/grant`)
    grant.search = url.search
    const linkFor = (identity) => {
      grant.searchParams.set('identity', identity)
      return grant.toString().replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;')
    }
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
    res.end(`<html><body><h1>Test identity provider</h1>
      <a data-test="idp-admin" href="${linkFor('admin')}">Continue as admin</a>
      <a data-test="idp-viewer" href="${linkFor('viewer')}">Continue as viewer</a>
    </body></html>`)
    return
  }
  if (url.pathname === '/grant') {
    if (url.searchParams.get('client_id') !== clientID || url.searchParams.get('redirect_uri') !== callback) {
      return sendJSON(res, 400, { error: 'invalid_request' })
    }
    const identity = url.searchParams.get('identity')
    if (identity !== 'admin' && identity !== 'viewer') {
      return sendJSON(res, 400, { error: 'invalid_identity' })
    }
    const code = randomBytes(24).toString('base64url')
    codes.set(code, {
      identity,
      nonce: url.searchParams.get('nonce'),
      challenge: url.searchParams.get('code_challenge'),
      expires: Date.now() + 60_000,
    })
    const redirect = new URL(callback)
    redirect.searchParams.set('code', code)
    redirect.searchParams.set('state', url.searchParams.get('state') || '')
    res.writeHead(302, { location: redirect.toString() })
    res.end()
    return
  }
  if (url.pathname === '/token' && req.method === 'POST') {
    let body = ''
    for await (const chunk of req) body += chunk
    const form = new URLSearchParams(body)
    const rawCode = form.get('code') || ''
    const code = codes.get(rawCode)
    codes.delete(rawCode)
    const verifier = form.get('code_verifier') || ''
    const challenge = createHash('sha256').update(verifier).digest('base64url')
    if (!code || code.expires < Date.now() || form.get('client_id') !== clientID ||
        form.get('client_secret') !== clientSecret || form.get('redirect_uri') !== callback ||
        form.get('grant_type') !== 'authorization_code' || challenge !== code.challenge) {
      return sendJSON(res, 400, { error: 'invalid_grant' })
    }
    return sendJSON(res, 200, { id_token: tokenFor(code), token_type: 'Bearer' })
  }
  sendJSON(res, 404, { error: 'not_found' })
})

server.listen(port, '127.0.0.1')
