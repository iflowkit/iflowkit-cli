# Getting started

Bu doküman, iFlowKit CLI'ı sıfırdan kurup ilk kez çalıştırmak için **kısa ama eksiksiz** bir akış verir.

- Tüm komutlar (tüm opsiyonlar + çok örnek) için: `cli-reference.md`
- Sync derin dokümanları için: `sync/*`

## 0) Build ve çalıştırma

```bash
go build ./cmd/iflowkit
./iflowkit help
```

## 1) Yerel config konumlarını gör

CLI kendi dosyalarını işletim sisteminin *user config* dizini altına yazar. Kendi makinenizdeki gerçek yolları görmek için:

```bash
iflowkit where
```

Bu komut şunları gösterir:

- Config root (örn. `.../iflowkit`)
- Profiles dizini
- `config.json` yolu
- `active_profile` yolu
- Log dizini

## 2) Developer config oluştur (önerilir)

`config.json` içinde şu anda tek önemli ayar vardır: `profileExportDir`.

```bash
iflowkit config init
iflowkit config show
```

## 3) Profile oluştur ve aktif et

Profil, Git remote kuralını ve tenant seviye sayısını (2 veya 3) belirler.

```bash
iflowkit profile init
iflowkit profile list

iflowkit profile use --id <profileId>
iflowkit profile current
iflowkit profile show
```

Not: Çoğu komut profili şu sırayla çözer:

1. `--profile <id>` verilmişse onu kullanır
2. Aksi halde `active_profile` dosyasını kullanır
3. Yoksa hata verir

## 4) Tenant service key ekle

Sync komutlarının çalışabilmesi için en az **DEV** tenant gerekir. QAS/PRD tenantları ise deliver/pull/push akışına göre gereklidir.

### 4.1) JSON dosyadan import

```bash
iflowkit tenant import --env dev --file service-key-dev.json
iflowkit tenant import --env qas --file service-key-qas.json
iflowkit tenant import --env prd --file service-key-prd.json

iflowkit tenant show --env dev
```

CLI'ın beklediği minimum JSON şekli:

```json
{
  "oauth": {
    "url": "https://<cpi-host>/",
    "tokenurl": "https://<oauth-host>/oauth/token",
    "clientid": "...",
    "clientsecret": "...",
    "createdate": "2026-01-06T10:00:00Z"
  }
}
```

### 4.2) Alanları komutla set et

```bash
iflowkit tenant set --env dev \
  --url https://<cpi-host>/ \
  --token-url https://<oauth-host>/oauth/token \
  --client-id <id> \
  --client-secret <secret>
```

## 5) Git token ayarla

`sync init` desteklenen provider'larda repo oluşturmayı denediği için bir token ister.

Tercih edilen tek değişken:

```bash
export IFLOWKIT_GIT_TOKEN="<token>"
```

GitHub fallback'leri: `GITHUB_TOKEN`, `GH_TOKEN`

GitLab fallback'leri: `GITLAB_TOKEN`, `GITLAB_PRIVATE_TOKEN`

## 6) Sync repo oluştur (DEV'den export)

IntegrationPackage id'sini biliyorsanız:

```bash
iflowkit sync init --id <packageId>
cd <packageId>
```

Farklı bir klasör altında oluşturmak için:

```bash
iflowkit sync init --id <packageId> --dir <parentPath>
cd <parentPath>/<packageId>
```

Komut sonunda repo içinde şu yapılar oluşur:

- `IntegrationPackage/` (CPI export içerikleri)
- `.iflowkit/package.json` (sync metadata)
- `.iflowkit/transports/...` (transport kayıtları)
- `.iflowkit/ignore` (ignore pattern dosyası)

## 7) Günlük sync akışları

### 7.1) Git -> CPI (push)

```bash
iflowkit sync push
```

Commit mesajına ek yapmak için:

```bash
iflowkit sync push --message "Update iFlow step"
```

PRD güvenliği: `prd` tenantına çalıştırmak için **mutlaka** `--to prd` gerekir:

```bash
git checkout prd
iflowkit sync push --to prd
```

Work branch örneği (DEV tenantına map edilir):

```bash
git checkout -b feature/new-flow
iflowkit sync push --message "WIP"
```

### 7.2) CPI -> Git (pull)

```bash
iflowkit sync pull
```

PRD güvenliği:

```bash
git checkout prd
iflowkit sync pull --to prd
```

Notlar:

- Working tree kirliyse (transport dosyaları hariç) CLI otomatik `git stash` yapabilir.
- Local branch origin'den gerideyse fast-forward yapar.
- Diverge olmuşsa durur ve sizden rebase/merge ile düzeltmenizi ister.

### 7.3) Branch karşılaştırma (compare)

```bash
iflowkit sync compare --to prd
```

3-tenant modelinde QAS compare da açılır:

```bash
iflowkit sync compare --to qas
```

### 7.4) Ortamlar arası promote (deliver)

3-tenant modelinde DEV -> QAS:

```bash
iflowkit sync deliver --to qas
```

PRD promote (2-tenant ise DEV -> PRD, 3-tenant ise QAS -> PRD):

```bash
iflowkit sync deliver --to prd
```

### 7.5) Deploy durumuna bak (deploy status)

```bash
iflowkit sync deploy status --env dev
```

Belirli bir transport id için:

```bash
iflowkit sync deploy status --env dev --transport <transportId>
```

## 8) Ignore patterns (.iflowkit/ignore)

Sync komutları diff hesaplarında ve object tespitinde `.iflowkit/ignore` dosyasını kullanır.

Varsayılan olarak şu pattern'ler zaten ignore edilir:

- `IntegrationPackage/**/metainfo.prop`
- `IntegrationPackage/**/src/main/resources/parameters.prop`

Detaylı söz dizimi ve örnekler için: `sync/ignore.md`

## 9) Loglar

Log seviyesi ve format global flag ile verilir:

```bash
iflowkit --log-level debug --log-format text where
```

Her çalışma ayrıca günlük bir dosyaya da yazar (konumu `iflowkit where` çıktısında bulunur).
