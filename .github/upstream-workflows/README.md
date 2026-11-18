# Inactive upstream workflows

These definitions are preserved outside `.github/workflows`, so GitHub
Actions does not discover or run them on branches containing this layout.
Their contents are unchanged from upstream.

| Files | Purpose |
| --- | --- |
| `docker.yml`, `proto-docker.yml` | Publish images to the upstream Cosmos container registry namespaces using Depot. |
| `build-simd-nightlies.yml` | Publish nightly simd binaries through GitHub Pages. |
| `proto-registry.yml` | Upstream protobuf registry publishing definition, already commented out upstream. |
| `sims.yml`, `sims-nightly.yml`, `sims-054.yml`, `sims-055.yml` | Scheduled or release-triggered simulation campaigns. |
| `clean-action-artifacts.yml` | Scheduled artifact deletion. |
| `stale.yml` | Scheduled stale issue and pull request maintenance. |

The fork does not currently operate these publishing and scheduled services.
PR builds, tests, lint, protobuf checks, and dependency checks remain in
`.github/workflows`.

To activate a definition later, configure its runners, publishing targets,
permissions, and credentials, then move the reviewed file back into
`.github/workflows`. Re-enable it in repository Actions settings if it was
also disabled there.

Upstream mirror branches retain upstream files. After synchronizing upstream,
review new or restored workflow files before enabling them for the fork.
