'use client'

import type { Profile } from '@remixproject/plugin-utils'
import { PluginClient } from '@remixproject/plugin'
import { createClient } from '@remixproject/plugin-webview'

class SentioClient extends PluginClient {
  client: any

  constructor() {
    super()
    if (typeof window !== 'undefined') {
      this.client = createClient(this)
    }
  }

  canDeactivate(from: Profile): boolean {
    return true
  }
}

const client = new SentioClient()

export default client
