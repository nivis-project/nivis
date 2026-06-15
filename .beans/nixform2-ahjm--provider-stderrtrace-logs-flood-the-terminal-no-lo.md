---
# nixform2-ahjm
title: Provider stderr/TRACE logs flood the terminal (no log suppression)
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-06-15T13:41:12Z
updated_at: 2026-06-15T13:45:45Z
parent: nixform2-djay
---

Found running against real AWS: go-plugin pipes the spawned provider's logs (hclog TRACE/DEBUG) to our stderr. AWS at TRACE during GetProviderSchema produced ~685 MB of log output, drowning the terminal and the real result. TF_LOG= empty did not suppress it. 
Fix: configure the go-plugin ClientConfig Logger to a quiet hclog (e.g. hclog.New with Level=hclog.Off or Warn, or Output=io.Discard), and/or set the provider's log level via the TF_LOG env we pass to the spawned process. Belongs with error-UX (E4c) polish. The fakes never showed this because they log almost nothing.



## Done
Fixed in OpenSpec large-provider-readiness: go-plugin ClientConfig.Logger set to
quiet hclog (Warn). AWS schema fetch stderr went from 685MB to 0 bytes.
