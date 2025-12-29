import * as monaco from 'monaco-editor'

export const parseUri = (uri?: monaco.Uri) => {
  if (!uri) {
    return {
      address: '',
      path: ''
    }
  }
  const pathList = uri.path.split('/')
  const address = pathList[1]
  const path = pathList.slice(2).join('/')
  return {
    address,
    path
  }
}
