---
id: practice.encryption-at-rest
type: practice
domain: security
title: Encryption at rest with managed keys
summary: Encrypt every store holding sensitive data, keep keys in a managed key service apart from the data, and plan rotation.
aliases: [data at rest, storage encryption, disk encryption, TDE, transparent data encryption, KMS, key vault, HSM, envelope encryption, DEK, KEK, key rotation, customer-managed keys, CMK]
coverage: checkable
applies_to:
  data_classes: [PHI, PII, confidential]
sources:
  - title: "OWASP Cryptographic Storage Cheat Sheet"
    url: https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html
    as_of: 2026-10-10
    licence_tier: facts-only
    copy: kb/sources/owasp/cryptographic-storage.md
  - title: "NIST SP 800-111, Guide to Storage Encryption Technologies for End User Devices"
    url: https://csrc.nist.gov/pubs/sp/800/111/final
    as_of: 2007-11-15
    licence_tier: public-domain
revised: 2026-10-10
---

## Practice

Any store that holds sensitive data is encrypted, and so are its replicas, snapshots and backups. Choose the layer from the threat: disk or volume encryption only helps against stolen hardware. Database- or application-level encryption also helps against someone who reaches the storage. Prefer AES with 256-bit keys in an authenticated mode.

Keys never live with the data they protect. Hold them in a managed key service (cloud KMS, key vault or HSM). Use envelope encryption: a data key encrypts the data, and a key-encryption key in the key service wraps the data key. Plan rotation before you need it, so a leaked key can be replaced quickly.

Storing less is the strongest control: don't keep sensitive data you don't need.

## How to check on a diagram

- Each Node with a sensitive Data Classification (stores, caches, queues) is marked encrypted, or uses a Service that encrypts by default.
- A key-management Node exists, and the stores that use customer-managed keys have an edge to it.
- Backups and replicas of a sensitive store are encrypted too.

## Suggested fixes

- Turn on encryption for the store and its backups.
- Add a KMS / Key Vault Node and link the stores that need customer-managed keys.

## Trade-offs

- Provider-managed keys are free and simple but give less control. Customer-managed keys add cost, a dependency on the key service, and rotation work.
- Application-level encryption protects most, but breaks database search and indexing on encrypted fields.
