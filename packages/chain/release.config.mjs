// Only packages/chain is published from this repo, so semantic-release runs with
// cwd=packages/chain and `semantic-release-monorepo` filters commits/notes down to
// this directory — otherwise every Go commit would cut a @sentio/chain release.
export default {
  extends: 'semantic-release-monorepo',
  // Not `v${version}`: that tag pattern triggers .github/workflows/ci.yml (bazel + image push).
  tagFormat: 'chain-v${version}',
  // main publishes straight to the `latest` dist-tag, matching what the manual
  // `pnpm pub` flow did. A prerelease-only branch list would fail with
  // ERELEASEBRANCHES, and an rc channel would need a `release` branch that
  // downstream consumers (which resolve `latest`) would then have to wait for.
  branches: ['main'],
  plugins: [
    [
      '@semantic-release/commit-analyzer',
      {
        preset: 'angular',
        releaseRules: [
          { type: 'release', scope: 'major', release: 'major' },
          { type: 'release', scope: 'minor', release: 'minor' },
          { type: 'release', scope: 'patch', release: 'patch' },
          { type: 'chore', release: 'patch' },
          { type: 'refactor', release: 'patch' },
        ],
      },
    ],
    [
      '@semantic-release/release-notes-generator',
      {
        preset: 'conventionalcommits',
        presetConfig: {
          types: [
            { type: 'feat', section: 'Features' },
            { type: 'fix', section: 'Bug Fixes' },
            { type: 'chore', section: 'Internal', hidden: false },
            { type: 'refactor', section: 'Internal', hidden: false },
          ],
        },
      },
    ],
    ['@semantic-release/github', { successComment: false, failComment: false, labels: false }],
  ],
}
