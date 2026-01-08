# CLI Reference

Bu doküman iFlowKit CLI'ın **tüm komutlarını** kapsar ve her komutu **opsiyonel parametreleriyle** birlikte örneklendirir.

> Not: Komut çıktıları CLI versiyonuna, tenant konfigürasyonuna ve git branch durumuna göre değişebilir.

## Global kullanım

Global flag'ler komut isminden **önce** verilir:

```bash
iflowkit [--profile <profileId>] [--log-level <trace|debug|info|warn|error>] [--log-format <text|json>] <command> [args]
```

### Global flag'ler

- `--profile <profileId>`
  - Birçok komut profil bağlamına ihtiyaç duyar.
  - Bu flag verilirse aktif profil yerine bunu kullanır.
- `--log-level trace|debug|info|warn|error` (varsayılan: `info`)
- `--log-format text|json` (varsayılan: `text`)

### Profil çözümleme (resolution) kuralları

Profil gerektiren komutlar profili şu sırayla çözer:

1. `--profile <id>` verilmişse onu kullanır
2. Aksi halde `active_profile` dosyasındaki id'yi kullanır
3. Yoksa hata verir (ör. `no profile selected; run iflowkit profile init ...`)

## help

Genel yardım çıktısı veya hedef komutun yardımını gösterir.

```bash
iflowkit help
iflowkit help <command>
iflowkit help <command> <subcommand>
```

Örnek:

```bash
iflowkit help profile export
iflowkit help sync push
```

## where

Yerel config yollarını ve profil bağlamını gösterir.

```bash
iflowkit where
```

Çıktı tipik olarak şunları içerir:

- Config root
- Profiles dizini
- `config.json` yolu
- `active_profile` yolu
- Logs dizini
- Aktif profil id ve resolve sonucu

## config

Developer tercihleri (`config.json`).

Dosya konumu: `.../iflowkit/config.json` (tam yolu `iflowkit where` ile görebilirsiniz).

### config init

Interactive wizard ile `config.json` oluşturur veya günceller.

```bash
iflowkit config init
```

Notlar:

- `profileExportDir` alanı zorunludur.
- Dizin yoksa, oluşturmayı sorabilir.

### config show

Mevcut `config.json` içeriğini basar.

```bash
iflowkit config show
```

### config export

`config.json` dosyasını bir `.iflowkit` arşivine export eder.

```bash
iflowkit config export
```

Opsiyonlar:

- `--out <file.iflowkit>` (varsayılan: `./config.iflowkit`)
- `--overwrite` (varsa hedef dosyayı sorusuz yazar)

Örnek:

```bash
iflowkit config export --out /tmp/config.iflowkit --overwrite
```

### config import

Bir `.iflowkit` config arşivini import eder.

```bash
iflowkit config import --file <file.iflowkit>
```

Opsiyonlar:

- `--overwrite` (varsa `config.json` üstüne sorusuz yazar)

Örnek:

```bash
iflowkit config import --file ./config.iflowkit --overwrite
```

## profile

Müşteri / proje profilleri.

Dosya konumu: `.../iflowkit/profiles/<profileId>/profile.json`

Profil alanları:

- `id` (dosya/dizin adı)
- `name`
- `gitServerUrl` (ör. `https://github.com`)
- `cpiPath` (ör. `/org-or-user/`)
- `cpiTenantLevels` (`2` veya `3`)

### profile init

Interactive wizard ile `profile.json` oluşturur.

```bash
iflowkit profile init
```

Opsiyonlar:

- `--overwrite` (varsa, üstüne yazma sorusunu atlar)

Örnek:

```bash
iflowkit profile init --overwrite
```

### profile list

Bulunan profilleri listeler.

```bash
iflowkit profile list
```

### profile current

Aktif profil id'sini ve resolve sonucunu gösterir.

```bash
iflowkit profile current
```

### profile show

Resolve edilen profilin `profile.json` içeriğini gösterir.

```bash
iflowkit profile show
```

Opsiyon:

- `--profile <profileId>` (başka bir profili göstermek için)

Örnek:

```bash
iflowkit profile show --profile acme
```

### profile use

Aktif profili ayarlar (`active_profile` dosyası).

```bash
iflowkit profile use --id <profileId>
```

### profile delete

Bir profili tamamen siler.

```bash
iflowkit profile delete --id <profileId> --yes
```

Not: `--yes` güvenlik için zorunludur.

### profile export

`profiles/<id>` klasörünü bir `.iflowkit` arşivine export eder.

```bash
iflowkit profile export --id <profileId>
```

Opsiyonlar:

- `--out <file.iflowkit>`
  - Verilmezse: önce `config.json.profileExportDir` denenir, yoksa current directory
- `--overwrite`

Örnek:

```bash
iflowkit profile export --id acme --out ./acme-profile.iflowkit --overwrite
```

### profile import

Bir `.iflowkit` profil arşivini import eder.

```bash
iflowkit profile import --file <profile-archive.iflowkit>
```

Opsiyonlar:

- `--overwrite` (varsa hedef profili sorusuz değiştirir)

Örnek:

```bash
iflowkit profile import --file ./acme-profile.iflowkit --overwrite
```

## tenant

Tenant servis anahtarları (service key) yönetimi.

Dosya konumu: `.../iflowkit/profiles/<profileId>/tenants/<env>.json`

Desteklenen ortamlar:

- `dev`
- `qas`
- `prd`

### tenant import

Bir service key JSON dosyasını okuyup tenant dosyasına yazar.

```bash
iflowkit tenant import --file <service-key.json> [--env dev|qas|prd]
```

Opsiyonlar:

- `--env dev|qas|prd` (varsayılan: `dev`)
- `--profile <profileId>` (aktif profil yerine)

Örnek:

```bash
iflowkit tenant import --env dev --file service-key-dev.json
```

### tenant show

Tenant dosyasını gösterir.

```bash
iflowkit tenant show [--env dev|qas|prd] [--profile <profileId>]
```

Örnek:

```bash
iflowkit tenant show --env prd
```

### tenant set

Service key alanlarını komut parametreleriyle set eder.

```bash
iflowkit tenant set --env dev|qas|prd \
  --url <url> \
  --token-url <tokenUrl> \
  --client-id <id> \
  --client-secret <secret> \
  [--created-at <rfc3339>]
```

Notlar:

- `--created-at` verilmezse CLI UTC zamanını kullanır.
- Değerler `profiles/<id>/tenants/<env>.json` altında saklanır.

### tenant delete

Bir environment için tenant dosyasını siler.

```bash
iflowkit tenant delete --env dev|qas|prd --yes [--profile <profileId>]
```

## sync

IntegrationPackage içeriğini Git ile senkronlamak için kullanılan modül.

Detaylı akışlar:

- Genel bakış: `sync/README.md`
- Ignore: `sync/ignore.md`
- Transport kayıtları: `sync/transports.md`

### Ön koşullar

- Repo tarafında `git` yüklü olmalı ve `PATH` üzerinde görünmeli.
- Profile aktif olmalı (veya `--profile` ile seçilmeli).
- Tenant key'leri import edilmiş olmalı:
  - `dev` her zaman gerekir
  - `qas` sadece `cpiTenantLevels=3` senaryosunda gerekir
  - `prd` prd branch / deliver için gerekir

### Branch -> tenant mapping

Sync, içinde çalıştığınız git branch'e göre tenant seçer:

- `dev` -> DEV tenant
- `qas` -> QAS tenant (sadece `cpiTenantLevels=3` ise)
- `prd` -> PRD tenant
- `feature/*` ve `bugfix/*` -> DEV tenant

### PRD güvenlik kuralı (`--to prd`)

PRD tenantına giden operasyonlarda yanlışlıkla çalıştırmayı önlemek için **ek onay** istenir:

- `git checkout prd` üzerinde olsanız bile `--to prd` vermeden PRD'e çalışmaz.

### sync init

DEV tenantından IntegrationPackage export ederek yeni bir repo oluşturur.

```bash
iflowkit sync init --id <packageId> [--dir <parentPath>]
```

Opsiyonlar:

- `--dir <parentPath>`: repo `<parentPath>/<packageId>` altında oluşturulur. Verilmezse current directory altında oluşur.

Örnek:

```bash
iflowkit sync init --id com.example.cpi.pkg
iflowkit sync init --id com.example.cpi.pkg --dir /tmp
```

### sync push

Git -> CPI akışı. Lokal değişiklikleri git'e commit/push eder ve değişen CPI artifact'lerini tenant'a uygular.

```bash
iflowkit sync push [--message <commitSuffix>] [--to dev|qas|prd]
```

Opsiyonlar:

- `--message <text>`: otomatik oluşturulan commit mesajlarının sonuna eklenir.
- `--to <env>`
  - PRD tenantında **zorunlu**: `--to prd`
  - Diğer tenantlarda opsiyoneldir; verildiyse hedef tenant ile eşleşmek zorundadır

Örnekler:

```bash
iflowkit sync push
iflowkit sync push --message "Refactor mapping"

git checkout prd
iflowkit sync push --to prd
```

### sync pull

CPI -> Git akışı. Hedef tenanttan IntegrationPackage export eder, repo içeriğini günceller ve origin/<branch>'e push eder.

```bash
iflowkit sync pull [--message <commitSuffix>] [--to dev|qas|prd]
```

Örnekler:

```bash
iflowkit sync pull
iflowkit sync pull --message "Refresh from CPI"

git checkout prd
iflowkit sync pull --to prd
```

### sync compare

Current branch ile target environment branch arasında IntegrationPackage farklarını listeler.

```bash
iflowkit sync compare --to qas|prd
```

Örnek:

```bash
iflowkit sync compare --to prd
```

### sync deliver

Ortamlar arası promote akışı.

```bash
iflowkit sync deliver --to qas|prd [--message <commitSuffix>]
```

Kurallar:

- `--to qas` sadece `cpiTenantLevels=3` için geçerlidir (DEV -> QAS)
- `--to prd`
  - `cpiTenantLevels=2`: DEV -> PRD
  - `cpiTenantLevels=3`: QAS -> PRD
- deliver çalışmadan önce hedef tenant ile hedef branch'in ignore sonrası eşit olması beklenir (güvenlik).

Örnek:

```bash
iflowkit sync deliver --to qas --message "Release candidate"
iflowkit sync deliver --to prd
```

### sync deploy status

Bir transport kaydındaki objeler için CPI runtime deploy durumunu gösterir.

```bash
iflowkit sync deploy status [--env dev|qas|prd] [--transport <transportId>]
```

Örnek:

```bash
iflowkit sync deploy status --env dev
iflowkit sync deploy status --env prd --transport 20260106T101530123Z
```
