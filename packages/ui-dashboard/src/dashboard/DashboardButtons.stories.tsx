import type { Story } from '@ladle/react'
import { DashboardButtonsMemo } from './DashboardButtons'

export const Editable: Story = () => (
  <div className="group p-8">
    <DashboardButtonsMemo
      allowEdit
      canExportCurl
      onMenuSelect={(k) => console.log('menu', k)}
    />
  </div>
)

export const FreeTierNoCurl: Story = () => (
  <div className="group p-8">
    <DashboardButtonsMemo
      allowEdit
      canExportCurl={false}
      exportCurlHint={<span>Upgrade to export as API.</span>}
      onMenuSelect={(k) => console.log('menu', k)}
    />
  </div>
)

export const Readonly: Story = () => (
  <div className="group p-8">
    <DashboardButtonsMemo
      allowEdit={false}
      onMenuSelect={(k) => console.log('menu', k)}
    />
  </div>
)
