# Güvenlik ve Gizlilik

Bu doküman, iFlowKit CLI kullanırken **service key / token / secret** gibi hassas bilgilerin nasıl yönetildiğini ve nelere dikkat edilmesi gerektiğini özetler.

> Bu proje henüz “enterprise secret management” yapmaz; doğru kullanım için bazı disiplinler gerekir.

---

## 1) CPI service key (tenant) dosyaları

### Nerede tutulur?

`iflowkit tenant import ...` ile içeri alınan bilgiler, kullanıcının config klasöründe saklanır:

- `os.UserConfigDir()/iflowkit/profiles/<profileId>/tenants/<env>.json`

Bu dosyada tipik olarak şunlar olur:

- CPI base URL
- OAuth token URL
- client id / client secret

### Öneriler

- Bu dosyaları **asla** git repo’larına commit etmeyin.
- CI/CD ortamında mümkünse service key’leri “secret store” üzerinden sağlayın.
- Bilgi sızıntısı şüpheniz varsa OAuth client secret’ı rotate edin.

---

## 2) Git provider token’ları

`sync init` bazı provider’larda repo oluşturmayı dener. Bunun için bir “personal access token” gerekir.

CLI token’ı environment variable’lardan çözer:

- Genel: `IFLOWKIT_GIT_TOKEN`
- GitHub fallback: `GITHUB_TOKEN` veya `GH_TOKEN`
- GitLab fallback: `GITLAB_TOKEN` veya `GITLAB_PRIVATE_TOKEN`

### Öneriler

- Token’ı “scope” olarak sadece ihtiyaç duyulan izinlerle verin (repo create/push).
- Token’ı shell history içine düşürmemek için export ederken dikkat edin.
- Paylaşımlı makinelerde environment variable’ları kalıcı profil dosyalarına yazmayın.

---

## 3) Log’lar

CLI logları şuraya yazar:

- `os.UserConfigDir()/iflowkit/logs/`

Log’lar, hata ayıklama için istek/cevap seviyesinde ayrıntı içerebilir.

### Öneriler

- `--log-level debug` kullanırken log’ların hassas veri içerebileceğini varsayın.
- Log klasörünü periyodik temizleyin (özellikle paylaşımlı makinelerde).
- Destek/issue açarken log paylaşacaksanız önce secret’ları maskeleyin.

---

## 4) Repo içindeki `.iflowkit/` klasörü

Sync repo içinde `.iflowkit/` altında:

- `package.json` (metadata)
- `ignore` (pattern’ler)
- `transports/` (transport kayıtları)

Bu klasör **service key** içermez; ancak operasyon geçmişi içerdiği için bazı kurumlarda yine de hassas kabul edilebilir.

### Öneriler

- `.iflowkit/transports/` içinde commit hash ve operasyon detayları bulunur.
- Bu kayıtlar, CPI secret’larını içermez; yine de gerektiğinde kurum politikanıza göre erişimi sınırlayın.

