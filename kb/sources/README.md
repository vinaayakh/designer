# Knowledge Base source copies

Reference copies of the sources that Records cite. They are used **only** by the CI copy check (`facts-only` sources must not share long verbatim runs with a Record). They are **never built into the binary**.

| File | Source | Licence | Retrieved |
|---|---|---|---|
| `hipaa/164-312.txt` | 45 CFR §164.312, CFR annual edition 2025-10-01 ([govinfo](https://www.govinfo.gov/content/pkg/CFR-2025-title45-vol2/xml/CFR-2025-title45-vol2-sec164-312.xml)) | US Government work, public domain | 2026-10-10 |
| `owasp/cryptographic-storage.md` | [OWASP Cryptographic Storage Cheat Sheet](https://github.com/OWASP/CheatSheetSeries/blob/cb9fe01e5ddf17b42513d2c53c4b86196c665c45/cheatsheets/Cryptographic_Storage_Cheat_Sheet.md) @ `cb9fe01` | CC BY-SA 4.0, © OWASP Foundation | 2026-10-10 |
| `aws/vpc-endpoints-s3.txt` | [Gateway endpoints for Amazon S3](https://docs.aws.amazon.com/vpc/latest/privatelink/vpc-endpoints-s3.html), body text extracted from HTML (code samples dropped) | CC BY-SA 4.0, © Amazon Web Services, Inc. | 2026-10-10 |
| `azure-architecture-center/cache-aside.md` | [Cache-Aside pattern](https://github.com/MicrosoftDocs/architecture-center/blob/0ab15d0b3533927509daf42f4ffac2af9916f786/docs/patterns/cache-aside-content.md) @ `0ab15d0` | CC BY 4.0, © Microsoft | 2026-10-10 |

Each third-party file keeps its own licence; the Knowledge Base licence (ADR 0002) does not apply to this folder.

Restricted sources (SOC 2 Trust Services Criteria, Azure Well-Architected) are **not** committed. Keep them in the gitignored `kb/.cache/sources/`; their check runs as a pre-commit step on the author's machine.
