# iFlowKit CLI Dokümantasyon

Bu klasör, **iflowkit CLI** için kullanıcı odaklı dokümantasyonu içerir.

## Başlangıç

- Hızlı kurulum ve ilk çalışma: `getting-started.md`
- Tüm komutlar (opsiyonel parametreler dahil) ve örnekler: `cli-reference.md`

## Sync (IntegrationPackage ↔ Git)

- Genel bakış, repo yapısı, branch/tenant modeli, güvenlik, retry: `sync/README.md`
- Sync komutları (tüm flag’ler ve örnekler): `sync/commands.md`
- `.iflowkit/ignore` (pattern söz dizimi ve örnekler): `sync/ignore.md`
- Transport kayıtları (retry mantığı, dosya formatı, tag isimlendirme): `sync/transports.md`
- Branch modeli ve promote akışları: `sync/branching.md`
- Sorun giderme (en sık hatalar ve çözümler): `sync/troubleshooting.md`

## Güvenlik / Gizlilik

- Token ve servis anahtarları, log’lar ve pratik öneriler: `security.md`
