const EXTENSION_EVENT = 'ExtensionInitialEvent'
const EXTENSION_HOOK_ID = 'sentioConnectHook'

export function ProjectPage() {
  const observer = new MutationObserver(function (mutations) {
    mutations.forEach(function (mutation) {
      if (mutation.type === 'attributes' && mutation.attributeName === 'id') {
        const newId = (mutation.target as Element).id
        if (newId === EXTENSION_HOOK_ID) {
          chrome.storage.sync.get('project').then((store) => {
            document.dispatchEvent(
              new CustomEvent(EXTENSION_EVENT, {
                detail: {
                  extensionId: chrome.runtime.id,
                  project: store.project
                }
              })
            )
          })
        }
      }
    })
  })

  observer.observe(document.documentElement, {
    attributes: true,
    subtree: true
  })
}
