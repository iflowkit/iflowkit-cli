# Sync Modülü (IntegrationPackage ↔ Git)

Bu doküman **`iflowkit sync`** modülünü “sıfırdan” anlatır.

`sync` modülü, SAP Cloud Integration (CPI) içindeki bir **IntegrationPackage**’ı **Git ile senkron** tutmak için tasarlanmıştır:

- CPI’dan paketi **export eder** → repo içinde `IntegrationPackage/` altına yazar.
- Repo’da yapılan değişiklikleri **Git’e push eder**, ardından CPI tenant’ında yalnızca etkilenen artefact’leri **update / delete / deploy** eder.
- Ortamlar arası promote sürecini (**dev → qas → prd**) branch merge + CPI güncellemesi ile tek bir “deliver” akışında yönetir.
- Her çalıştırmada `.iflowkit/transports/` altında **audit + retry** amaçlı transport kayıtları yazar.

> Bu modül “tam otomatik” olmaktan ziyade, CPI ekiplerinin zaten yaptığı işleri (export, git commit/push, deploy, promote) **deterministik, izlenebilir ve tekrar çalıştırılabilir** hale getirmeyi hedefler.

## İlgili dokümanlar

- Komutlar ve tüm opsiyonlar: `commands.md`
- Ignore mantığı ve pattern dili: `ignore.md`
- Transport kayıt formatı + retry modeli: `transports.md`
- Branch modeli ve önerilen akışlar: `branching.md`
- Sorun giderme: `troubleshooting.md`

---

## Temel model: “iFlowKit neyi senkronlar?”

### 1) Kaynak gerçekliği

`sync`, repo içinde aşağıdaki dizini **paketin içerik gerçeği** olarak kabul eder:

- `IntegrationPackage/`  (varsayılan)

Bu klasörün adı `.iflowkit/package.json` içindeki `baseFolder` ile belirlenir; pratikte varsayılan `IntegrationPackage` kullanılır.

> `sync` komutları artefact’leri “dosya bazında” değil, **artefact bazında** ele alır.
> Yani bir artefact’in içindeki tek bir dosyada değişiklik olsa bile, CPI tarafında upload/deploy işlemi genellikle o artefact’in tamamına uygulanır.

### 2) Artefact türleri (destek)

Kod tarafında “değişiklik tespiti” şu kind klasörleriyle çalışır:

- `iFlows`
- `Scripts`
- `ValueMappings`
- `MessageMappings`
- `CustomTags` *(diff’te görünür; CPI update/delete tarafı sınırlı olabilir)*

Artefact kimliği repo içinde şu path düzeninden çıkarılır:

```
<baseFolder>/<Kind>/<ArtifactId>/...
# örnek
IntegrationPackage/iFlows/MyFlow/META-INF/...
```

### 3) Branch → tenant eşlemesi

`sync` hangi CPI tenant’ına çalışacağını **git branch**’ten çözer.

| Branch | Tenant | Not |
|---|---|---|
| `dev` | `dev` | environment branch |
| `qas` | `qas` | sadece `cpiTenantLevels=3` ise |
| `prd` | `prd` | environment branch |
| `feature/*` | `dev` | work branch |
| `bugfix/*` | `dev` | work branch |

Kritik sonuç:

- `sync pull` sadece **environment branch**’lerde çalışır (dev/qas/prd).
- `sync push` hem environment branch’lerde hem de work branch’lerde çalışabilir.

---

## Güvenlik kuralı: PRD onayı

PRD’e yanlışlıkla yazmayı engellemek için ek bir onay mekanizması vardır.

Aşağıdaki komutlarda **`--to prd` vermeden** PRD tenant’ına gidilmez:

- `iflowkit sync push --to prd`
- `iflowkit sync pull --to prd`
- `iflowkit sync deliver --to prd` *(deliver’da `--to` zaten hedefi belirlediği için aynı zamanda onaydır)*

---

## Repo yapısı

`sync init` ile oluşan minimum yapı:

```text
<packageId>/
  IntegrationPackage/
    ... CPI export içerikleri ...
  .iflowkit/
    package.json
    ignore
    transports/
      dev/
        index.json
        <transportId>.transport.json
  .gitignore
```

### `.iflowkit/package.json`

Repo’nun “sync repo” olduğunu belirleyen metadatadır.

Önemli alanlar:

- `profileId`: bu repo hangi profile ile ilişkilendirildi
- `cpiTenantLevels`: 2 veya 3 (env modeli)
- `packageId`, `packageName`
- `baseFolder`: varsayılan `IntegrationPackage`
- `gitRemote`, `gitProvider`

### `.iflowkit/ignore`

Ignore file, *diff hesaplamasında* ve bazı güvenlik kontrollerinde kullanılır.

- Pattern dili: `ignore.md`
- Not: ignore, Git’in `.gitignore` mantığı değildir; **sync’in artefact seçimi** için kullanılır.

### `.iflowkit/transports/`

Her tenant env için ayrı transport kayıtları tutulur:

- `.iflowkit/transports/dev/`
- `.iflowkit/transports/qas/`
- `.iflowkit/transports/prd/`

Bu kayıtlar:

- “Bu çalıştırmada hangi artefact’ler değişti?” bilgisini saklar.
- CPI tarafında hata olursa “kaldığı yerden devam” edebilmek için **remaining listeleri** tutar.

Detay: `transports.md`

---

## Çalışma mantığı (yüksek seviye)

### `sync init` (DEV → yeni repo)

- DEV tenant’tan paketi export eder
- Repo’yu oluşturur ve `dev` branch’ine push eder
- `.iflowkit/package.json`, `.iflowkit/ignore`, `.iflowkit/transports/...` yazar

### `sync pull` (CPI → Git)

- Sadece environment branch’lerde (dev/qas/prd) çalışır
- CPI’dan export alır ve `IntegrationPackage/` klasörünü **yeniden yazar**
- Yerel/remote branch durumunu kontrol eder:
  - Remote’dan gerideyseniz `--ff-only` fast-forward yapar
  - Local ve remote diverged ise durur
- Çalışma ağacında değişiklik varsa (transport kayıtları hariç) **stash** yapar
- Export sonrası `IntegrationPackage/` commit eder ve `origin/<branch>`’e push eder
- CPI’dan silinen artefact’leri repo’dan da silmiş olur (folder tamamen yeniden yazıldığı için)

### `sync push` (Git → CPI)

- Environment branch’lerde (dev/qas/prd) veya work branch’lerde (feature/*, bugfix/*) çalışır
- Önce Git tarafını “temiz ve push edilmiş” hale getirir:
  - `IntegrationPackage/` değişikliklerini *strict formatlı* bir contents commit’e alır
  - branch’i `origin`’e push eder
- Sonra CPI tarafına uygular:
  1) Repo’dan silinen artefact’leri CPI’dan siler (desteklenen kind’lerde)
  2) Değişen artefact’leri CPI’a upload eder
  3) Upload edilen artefact’leri deploy eder (iFlow/script/mapping)
- CPI adımlarının her birinde `.iflowkit/transports/...` kaydı güncellenir; hata olursa pending kalır

### `sync deliver` (promote)

- `--to qas|prd` ile çalışır
- Önce hedef branch ile hedef tenant’ın **eşit** olduğundan emin olur (ignore uygulanmış diff ile)
  - Eşit değilse “promote” yapmayı durdurur (güvenlik)
- Sonra source → target merge yapar:
  - 3-level: `dev → qas`, ardından `qas → prd`
  - 2-level: `dev → prd`
- Merge sonrası oluşan farklardan etkilenen artefact setini hesaplar
- Bu seti CPI target tenant’ına “delete/upload/deploy” olarak uygular

### `sync compare` (branch farkı)

- `--to qas|prd` ile, current branch ile `origin/<to>` arasında diff çıkarır
- Ignore pattern’lerini uygular
- Çıktı: `Kind - ObjectId`

### `sync deploy status` (runtime durumu)

- Local transport kaydındaki `objects` listesine bakar
- CPI runtime’dan deploy status sorgular ve tablo basar

---

## Silme (delete) davranışı

Silme iki farklı yönden tetiklenir:

### 1) CPI tarafında silme → repo’dan silme (pull)

`sync pull` çalışırken:

- Export öncesi repo’daki mevcut artefact envanterini okur
- Export sonrası yeni envanterle karşılaştırır
- Export çıktısında artık olmayan artefact klasörleri repo’dan kaldırılmış olur

Bu silmeler `transportRecord.deletedObjects` alanına yazılır.

### 2) Repo’da silme → CPI’dan silme (push/deliver)

`sync push` veya `sync deliver` sırasında:

- Diff ile “değişmiş artefact” seti çıkarılır
- Eğer ilgili artefact klasörü artık yoksa, bu değişiklik *deletion* olarak işaretlenir
- CPI delete işlemi kind bazında desteklenir:
  - desteklenen: `iFlows`, `Scripts`, `ValueMappings`, `MessageMappings`
  - desteklenmeyenlerde delete no-op olabilir

Bu silmeler `transportRecord.deletedObjects` alanına yazılır.

---

## Retry (kaldığı yerden devam) modeli

CPI tarafında upload/deploy işlemleri sırasında ağ/timeout/tenant hataları olabilir.

`sync`, bu durumda “ne yapıyordum?” bilgisini `.iflowkit/transports/<env>/...transport.json` içinde tutar.

Pratikte retry şöyle çalışır:

1) `sync push` (veya deliver) CPI tarafında hata verir → transport `pending` kalır
2) Aynı branch’te **aynı komutu tekrar çalıştırırsınız**
3) iFlowKit en son pending kaydı bulur ve `uploadRemaining/deleteRemaining/deployRemaining` listelerinden devam eder

Detay alanlar ve örnek JSON: `transports.md`

---

## Günlük kullanım için önerilen “altın akışlar”

### A) İlk kurulum (bir kez)

```bash
iflowkit config init
iflowkit profile init
iflowkit profile use --id <profileId>

iflowkit tenant import --env dev --file ./service-key-dev.json
# cpiTenantLevels=3 ise
iflowkit tenant import --env qas --file ./service-key-qas.json
# prd işlemleri yapacaksanız
iflowkit tenant import --env prd --file ./service-key-prd.json
```

### B) Repo oluşturma

```bash
iflowkit sync init --id com.example.cpi.pkg
cd com.example.cpi.pkg
```

### C) DEV’de günlük geliştirme

```bash
git checkout dev
iflowkit sync pull

# değişiklik yap (IntegrationPackage altında)

iflowkit sync push --message "Update mapping"
```

### D) Feature branch ile izolasyon

```bash
git checkout -b feature/new-flow

# değişiklik yap

# feature branch, DEV tenant’a yazacağı için PRD güvenlik flag’i gerekmez
iflowkit sync push --message "WIP"

# sonra PR ile dev’e merge
```

### E) Promote (deliver)

```bash
# 3-level: dev -> qas
iflowkit sync deliver --to qas --message "RC"

# prd promote:
# - 2-level: dev -> prd
# - 3-level: qas -> prd
iflowkit sync deliver --to prd --message "Release"
```
