# Backend RBAC Matrix
Roles: `OPERATOR`, `SUPERVISOR`. `Public` means no JWT. `Authenticated` means either role. All checks enforced by Gin middleware **and** sensitive service logic.

| Operation | Operator | Supervisor |
|---|---|---|
| Login / refresh | Public/session | Public/session |
| Logout | Yes | Yes |
| Product list | Yes | Yes |
| Product create | **No** | Yes |
| RCV list/detail | Yes | Yes |
| RCV create | Yes | **No** (baseline responsibility) |
| TRX list/detail | Yes | Yes |
| TRX create | Yes | **No** (baseline responsibility) |
| TRX approve | **No** | Yes |
| Rejected TRX correction | **No** | Yes |
| Rejected TRX review resolution | **No** | Yes |
| Inventory list/ledger | Yes | Yes |
| Receiving audit list | **No** | Yes |
| Own notifications list/read | Yes, own only | Yes, own only |
| Other user's notifications | **No** | **No** |

- Return 401 without valid authentication, 403 for authenticated role denial. Both use `{"success":false,"error":{"code":"...","message":"..."}}`; role-denial code is `forbidden`.
- Enforce ownership on notification queries/updates in SQL, not merely frontend filters. A notification outside the authenticated recipient scope returns 404 with code `not_found`.
- Check session revocation and token validity according to `/home/tyomaans/Projects/auth-session`.
- A supervisor may inspect Operator-created RCV/TRX but must not impersonate Operator to submit receiving.
- Audit actors must come from authenticated identity, never from request-provided user IDs.
- Login is public. Refresh requires both HttpOnly `refresh_token` and `session_id` cookies; logout additionally requires a live Bearer JWT and succeeds with an empty HTTP 204 response. Cookie paths and attributes are locked in `docs/technical-spec.md` and `docs/openapi.yml`.
- Redis unavailability returns generic 503, never authenticated access. Successful logout/revocation invalidates the current session's existing access JWTs immediately.
- Approval records approved_by/approved_at; correction records corrected_by/corrected_at and an immutable reason/before/after audit; resolution records reviewed_by/reviewed_at and review_note. Receiving creation/submission records created_by/created_at. All IDs/times are supplied by the server.
