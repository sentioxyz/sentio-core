export function sentioTxUrl(chainId, hash) {
  return `https://app.sentio.xyz/tx/${chainId}/${hash}`
}

export function sentioSimUrl(chainId, hash) {
  return `https://app.sentio.xyz/sim/${chainId}/${hash}`
}

export function sentioProjectSimUrl(
  chainId: string,
  hash: string,
  projectOwner: string,
  projectSlug: string
) {
  return `https://app.sentio.xyz/${projectOwner}/${projectSlug}/simulator/${chainId}/${hash}`
}

export function sentioContractUrl(chainId, address) {
  return `https://app.sentio.xyz/contract/${chainId}/${address}?t=1`
}

export function sentioProjectSimulatorUrl(projectPath: string) {
  return `https://app.sentio.xyz/${projectPath}/simulator`
}

export function sentioCompilationUrl(compilationId: string) {
  return `https://app.sentio.xyz/compilation/${compilationId}`
}

export function ethscriptionUrl(hash: string) {
  return `https://ethscriptions.com/ethscriptions/${hash}`
}
