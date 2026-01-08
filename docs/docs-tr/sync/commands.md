# Sync Komutları (Detaylı Referans)

Bu doküman **`iflowkit sync`** komutlarını, tüm opsiyonel parametreleri ve gerçekçi örneklerle birlikte açıklar.

> Kısa komut özeti için `../cli-reference.md` dosyasına da bakabilirsiniz. Bu doküman sync tarafını daha “operasyonel” seviyede anlatır.

---

## Hızlı özet: hangi komut ne yapar?

| Komut | Ne yapar? | Nerede çalışır? |
|---|---|---|
| `sync init` | DEV tenant’tan bir IntegrationPackage export eder, yeni git repo oluşturur ve `dev` branch’e push eder | herhangi bir klasör (hedef dizin boş olmalı) |
| `sync pull` | CPI → Git: bulunduğunuz environment branch’e göre tenant’tan export alır, repo’yu günceller ve `origin/<branch>`’e push eder | sync repo içinde, **dev/qas/prd** |
| `sync push` | Git → CPI: IntegrationPackage değişikliklerini git’e push eder, sonra tenant’ta artefact’leri update/delete/deploy eder | sync repo içinde, **dev/qas/prd** + `feature/*` + `bugfix/*` |
| `sync compare` | Current branch ile `origin/<to>` (qas/prd) arasında “artefact farkı” listeler (ignore uygulanır) | sync repo içinde |
| `sync deliver` | Promote: dev→qas veya dev/qas→prd merge eder, target branch’e push eder ve target tenant’ı günceller | sync repo içinde |
| `sync deploy status` | Seçilen transport kaydındaki objelerin CPI runtime deploy durumunu gösterir | sync repo içinde |

---

## Ortak kavramlar

### Branch → tenant çözümleme

- `dev`, `qas`, `prd` branch’leri “environment branch” kabul edilir.
- `feature/*` ve `bugfix/*` branch’leri “work branch” kabul edilir ve **DEV tenant**’a map’lenir.

### PRD güvenlik kuralı

PRD tenant’ına giden operasyonlarda yanlışlıkla çalıştırmayı önlemek için:

- `sync push` / `sync pull` için **`--to prd`** verilmesi gerekir.
- `sync deliver --to prd` zaten hem hedefi hem onayı içerir.

### Ignore dosyası

Ignore pattern’leri `.iflowkit/ignore` dosyasından okunur ve bazı karar noktalarında kullanılır:

- `push`: hangi artefact’lerin “değişti” sayılacağını belirler
- `compare`: listelenen farkları filtreler
- `deliver`: “tenant == target branch” preflight kontrolünde diff hesaplamasını filtreler

Detay: `ignore.md`

---

## `iflowkit sync init`

DEV tenant’tan bir IntegrationPackage export ederek **yeni bir sync repo** oluşturur.

### Kullanım

```bash
iflowkit sync init --id <packageId> [--dir <parentPath>]
```

### Parametreler

- `--id <packageId>` (**zorunlu**)
  - CPI IntegrationPackage id’si (ör. `com.example.cpi.pkg`).
- `--dir <parentPath>` (opsiyonel)
  - Repo `<parentPath>/<packageId>` altında oluşturulur.
  - Verilmezse current directory altında `<packageId>/` oluşturulur.

### Ön koşullar

- Aktif profil seçilmiş olmalı (`iflowkit profile use --id <profileId>`), ya da global `--profile` ile verilmelidir.
- DEV tenant service key import edilmiş olmalı:

```bash
iflowkit tenant import --env dev --file service-key-dev.json
```

- Git provider repo yaratımı için token (önerilir):
  - `IFLOWKIT_GIT_TOKEN`
  - GitHub fallback: `GITHUB_TOKEN` veya `GH_TOKEN`
  - GitLab fallback: `GITLAB_TOKEN` veya `GITLAB_PRIVATE_TOKEN`

### Ne üretir?

`sync init` tipik olarak şunları oluşturur:

- `IntegrationPackage/` (CPI export içeriği)
- `.iflowkit/package.json` (sync metadata)
- `.iflowkit/ignore` (ignore template + default pattern’ler)
- `.iflowkit/transports/dev/...` (init transport kaydı)
- `.gitignore`
- `git init`, `dev` branch, initial push

### Örnekler

```bash
# repo current directory altında açılır: ./com.example.cpi.pkg
iflowkit sync init --id com.example.cpi.pkg

# repo belirli bir parent path altında açılır: /tmp/com.example.cpi.pkg
iflowkit sync init --id com.example.cpi.pkg --dir /tmp
```

---

## `iflowkit sync pull`

CPI → Git akışı.

Bulunduğunuz environment branch’e göre ilgili tenant’tan IntegrationPackage export eder, `IntegrationPackage/` içeriğini yeniler ve sonucu `origin/<branch>`’e push eder.

### Kullanım

```bash
iflowkit sync pull [--to dev|qas|prd] [--message <commitSuffix>]
```

### Parametreler

- `--message <text>` (opsiyonel)
  - Otomatik oluşturulan commit mesajlarının sonuna eklenir.
- `--to <env>` (opsiyonel; **prd için zorunlu**)
  - Hedef tenant ile branch mapping’in aynı olduğunu “onaylar”.
  - PRD’de `--to prd` verilmezse komut çalışmaz.

### Branch kuralları

- Sadece `dev`, `qas`, `prd` branch’lerinde çalışır.
- `qas` branch’i sadece `cpiTenantLevels=3` ise anlamlıdır.

### Git davranışı (önemli)

`sync pull`:

1. `origin/<branch>`’i fetch eder.
2. Local branch remote’dan **gerideyse** `--ff-only` ile fast-forward yapar.
3. Local branch remote ile **diverged** ise durur (rebase/merge bekler).
4. Working tree’de değişiklik varsa (transport log’ları hariç) güvenli olmak için **stash** eder.

### CPI davranışı

- CPI’dan export alır.
- `IntegrationPackage/` klasörünü önce siler, sonra export’u yazar.
  - Bu sayede CPI’da silinmiş artefact’ler repo’dan da silinmiş olur.

### Transport kaydı

- `.iflowkit/transports/<env>/` altına `transportType=pull` bir kayıt yazar.
- Kayıtta `deletedObjects` alanı, CPI’da silinmiş artefact’leri (repo’dan da silinenleri) listeler.

### Örnekler

```bash
# dev branch
git checkout dev
iflowkit sync pull

# qas branch (3-tenant model)
git checkout qas
iflowkit sync pull

# prd branch (safety)
git checkout prd
iflowkit sync pull --to prd

# commit mesajına suffix eklemek
iflowkit sync pull --message "Refresh from CPI"
```

---

## `iflowkit sync push`

Git → CPI akışı.

Repo’daki değişiklikleri önce Git’e push eder, sonra tenant’ta yalnızca “değişmiş” sayılan artefact’leri **update/delete/deploy** eder.

### Kullanım

```bash
iflowkit sync push [--to dev|qas|prd] [--message <commitSuffix>]
```

### Parametreler

- `--message <text>` (opsiyonel)
  - Otomatik commit mesajlarının sonuna eklenir.
- `--to <env>` (opsiyonel; **prd için zorunlu**)
  - Hedef tenant onayı.

### Branch kuralları

- Environment branch’ler: `dev`, `qas`, `prd`
- Work branch’ler: `feature/*`, `bugfix/*`

Work branch’lerde hedef tenant **dev**’dir.

### Değişiklik tespiti nasıl çalışır?

`sync push` değişiklikleri şu şekilde bulur:

1. Git üzerinden “branch’in upstream’ine göre” değişen path’leri çıkarır.
2. Bu path listesine `.iflowkit/ignore` uygulanır.
3. Kalan path’ler `IntegrationPackage/<Kind>/<Id>/...` formatındaysa ilgili artefact “changed” kabul edilir.
4. Artefact dizini yerelde **yoksa** ve artefact türü silinebilir bir türse (iFlows, Scripts, ValueMappings, MessageMappings) bu artefact “delete” kabul edilir.

> Not: `CustomTags` gibi bazı türler diff’te görünse bile CPI update/delete yolu sınırlı olabilir. Bu durumda komut uyarı verip ilgili işi pas geçebilir.

### Retry / pending mantığı

CPI aşamasında bir hata olursa:

- Transport kaydı `transportStatus=pending` olarak kalır.
- Kayıttaki `uploadRemaining` / `deleteRemaining` / `deployRemaining` alanları, **kalan işleri** listeler.
- Aynı branch/tenant üzerinde `sync push` tekrar çalıştırıldığında, en son pending kaydı bulur ve **kaldığı yerden devam eder**.

### Git tag

- `sync push` başarılı olursa ve branch environment branch ise (`dev/qas/prd`), `<transportId>_<branch>` formatında tag oluşturur ve remote’a push eder.
- Work branch’lerde tag oluşturulmaz.

### Örnekler

```bash
# dev branch
git checkout dev
iflowkit sync push

# prd branch (safety)
git checkout prd
iflowkit sync push --to prd

# work branch (DEV tenant'a yazar)
git checkout -b feature/new-flow
# değişiklikleri yap...
iflowkit sync push --message "WIP"
```

---

## `iflowkit sync compare`

Current branch ile `origin/<to>` arasında artefact farklarını listeler (ignore uygulanır).

### Kullanım

```bash
iflowkit sync compare --to qas|prd
```

### Parametreler

- `--to qas|prd` (**zorunlu**)
  - Hedef environment branch.

### Davranış

- `git diff --name-only origin/<to>..HEAD -- <baseFolder>` ile path farklarını alır.
- `.iflowkit/ignore` uygulayarak gürültüyü temizler.
- Kalan path’lerden artefact setini çıkarır ve `Kind - Id` formatında basar.

### Örnekler

```bash
# dev'de çalışırken qas'a göre fark
iflowkit sync compare --to qas

# qas -> prd farkı için (qas branch'teyken)
iflowkit sync compare --to prd
```

---

## `iflowkit sync deliver`

Ortamlar arası promote akışı.

`deliver` iki işi birlikte yapar:

1. Branch merge (source → target)
2. Target tenant’ı, merge sonucu oluşan artefact değişiklik setiyle günceller (delete/upload/deploy)

### Kullanım

```bash
iflowkit sync deliver --to qas|prd [--message <commitSuffix>]
```

### Parametreler

- `--to qas|prd` (**zorunlu**)
- `--message <text>` (opsiyonel)

### Promote yolu

- `cpiTenantLevels=3`:
  - `--to qas`  → `dev → qas`
  - `--to prd`  → `qas → prd`
- `cpiTenantLevels=2`:
  - `--to prd`  → `dev → prd`

### Preflight kontroller

- Working tree **temiz** olmalı.
- Target tenant ile target branch’in içeriği **eşit** olmalı (ignore uygulanır).
  - Amaç: PRD/QAS tenant’ına “beklenmeyen” drift varken promote’u engellemek.

### Branch bootstrap

Eğer `origin/qas` veya `origin/prd` yoksa:

- iFlowKit branch’i **tenant’tan export ederek** bootstraps eder.
- Bu bootstrap sırasında `transportType=init` bir kayıt ve bir git tag oluşabilir.

### Retry / pending

Deliver sırasında CPI tarafında bir hata oluşursa:

- Transport kaydı `pending` kalır.
- `deliver` tekrar çalıştırıldığında, en son pending deliver kaydını bulur ve **kaldığı yerden CPI işini tamamlamaya** çalışır.

### Örnekler

```bash
# dev -> qas (3-tenant)
iflowkit sync deliver --to qas --message "Release candidate"

# prd promote
# - 2-tenant: dev -> prd
# - 3-tenant: qas -> prd
iflowkit sync deliver --to prd
```

---

## `iflowkit sync deploy status`

Bir transport kaydındaki artefact’lerin CPI runtime deploy durumunu gösterir.

### Kullanım

```bash
iflowkit sync deploy status [--env dev|qas|prd] [--transport <transportId>]
```

### Parametreler

- `--env dev|qas|prd` (varsayılan: `dev`)
- `--transport <id>` (opsiyonel)
  - Verilmezse ilgili env altında en son transport kaydı seçilir.

### Çıktı

Tablo formatında basar:

- KIND
- NAME
- STATUS
- DEPLOYED_AT

### Örnekler

```bash
# dev env altında en son transport'un deploy durumu
iflowkit sync deploy status

# prd env altında belirli bir transport'u kontrol
iflowkit sync deploy status --env prd --transport 20260106T101530123Z
```
