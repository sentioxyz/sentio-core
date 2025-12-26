export function isSentioPage() {
  const { host } = document.location

  return ['localhost:10000', 'test.sentio.xyz', 'app.sentio.xyz'].includes(host)
}
