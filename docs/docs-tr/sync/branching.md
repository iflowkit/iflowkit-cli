# Branch Modeli ve Ortam Akışları

Bu doküman, `iflowkit sync` modülünün **branch → tenant** yaklaşımını ve özellikle **promote / deliver** akışını nasıl kurguladığını anlatır.

> `sync` modülü “branch’i ortam olarak” düşünür. Bu sayede hem Git tarafında hem CPI tarafında aynı isimlerle ilerler: `dev`, `qas`, `prd`.

---

## Temel kavramlar

### Environment branch

Aşağıdaki branch’ler “environment branch” kabul edilir:

- `dev`
- `qas` *(yalnızca `cpiTenantLevels=3` ise)*
- `prd`

Bu branch’lerin temel özellikleri:

- CPI tenant’a **birebir map** edilir.
- `sync pull` yalnızca bu branch’lerde çalışır.
- `sync push` bu branch’lerde başarılı olursa **git tag** oluşturur.

### Work branch

Aşağıdaki branch paternleri “work branch” kabul edilir:

- `feature/*`
- `bugfix/*`

Özellikler:

- Tenant mapping her zaman **DEV**’dir.
- Amaç: izole geliştirme.
- `sync pull` çalışmaz (çünkü work branch “ortam” değil).
- `sync push` çalışır ve CPI DEV’e yazar.

---

## Branch → tenant eşlemesi (kesin tablo)

| Branch | Tenant |
|---|---|
| `dev` | `dev` |
| `qas` | `qas` *(3-level)* |
| `prd` | `prd` |
| `feature/*` | `dev` |
| `bugfix/*` | `dev` |

---

## Önerilen branching stratejisi

### Senaryo A: 3 tenant (dev → qas → prd)

Bu en yaygın kurumsal modeldir:

1. Geliştirici `feature/*` branch açar ve DEV üzerinde iteratif çalışır (`sync push`).
2. PR/merge ile değişiklikler `dev` branch’e alınır.
3. `dev` branch’in CPI DEV ile eşitlenmesi gerekiyorsa `sync pull` yapılır.
4. Test/QA için `dev → qas` promote edilir: `sync deliver --to qas`.
5. Üretim için `qas → prd` promote edilir: `sync deliver --to prd`.

Örnek akış:

```bash
# feature branch
git checkout -b feature/order-routing
# dosyaları değiştir...
iflowkit sync push --message "WIP"

# dev branch’e merge sonrası
git checkout dev
git pull
iflowkit sync pull

# QA promote
iflowkit sync deliver --to qas --message "RC-1"

# PROD promote
iflowkit sync deliver --to prd --message "Release 1.0"
```

### Senaryo B: 2 tenant (dev → prd)

Bu modelde `qas` yoktur:

- Promote: `dev → prd`

```bash
git checkout dev
iflowkit sync pull

# üretime promote
iflowkit sync deliver --to prd --message "Release 1.0"
```

---

## `deliver` akışı “tam olarak” ne yapar?

`iflowkit sync deliver --to <qas|prd>` iki işi tek komutta birleştirir:

1. **Git promote**: source branch’ten target branch’e merge (no-ff)
2. **CPI promote**: oluşan fark setini target tenant’a upload/delete/deploy ederek uygular

### Promote yolu nasıl seçilir?

- `--to qas` (3-level): `dev → qas`
- `--to prd`:
  - 2-level: `dev → prd`
  - 3-level: `qas → prd`

### Target branch yoksa ne olur?

`deliver`, ihtiyaç duyduğu environment branch’ler için `origin/<env>` var mı diye kontrol eder.

Eksikse otomatik “bootstrap” yapar:

- tenant’tan export alır
- branch’i oluşturur ve `origin/<env>`’e push eder
- bir “init transport” kaydı üretir
- **git tag** üretir (bootstrap transportId ile)

Bu davranış özellikle yeni başlatılan projelerde çok iş görür: “qas/prd branch’leri henüz yok ama tenant’ta içerik var” senaryosu.

---

## Çalışma prensipleri ve güvenlik

### 1) Clean working tree zorunluluğu

`deliver` çalışmadan önce repo’nun *çalışma ağacının temiz* olmasını ister.

Sebep: merge sırasında istemeden local dosya değişikliklerini taşımamak.

Çözüm:

- `git status` ile kontrol et
- commit / stash yap

### 2) Target tenant == target branch preflight

`deliver` yeni bir transport başlatmadan önce şu kontrolü yapar:

- target tenant’tan export al
- target branch’in `IntegrationPackage/` içeriği ile karşılaştır
- `.iflowkit/ignore` uygulanmış haliyle eşitse devam et

Eşit değilse komut hata verir.

Bu, “branch zaten tenant’ta uygulanmamış değişiklik içeriyor” veya “tenant’da manuel değişiklik yapılmış” gibi durumlarda yanlış promote’u engeller.

### 3) PRD onayı

PRD’e giden operasyonlar için ekstra onay vardır:

- `deliver` için bu onay zaten `--to prd` parametresidir.

---

## “Ben PR merge kullanıyorum, deliver’a ihtiyaç var mı?”

Eğer Git tarafında environment branch’e merge/PR akışınız oturmuşsa:

- `sync deliver`’ı sadece “tenant’a uygulama” adımı için kullanmak isteyebilirsiniz.

Ancak `deliver` git merge’i de içerdiği için iki seçenek var:

1. **Tek komut promote**: merge + tenant update (önerilen, deterministik)
2. **Ayrı ayrı**: PR/merge ile branch’i güncelle, sonra tenant update için `sync push` (hedef branch’te)

İkinci yaklaşımda da PRD güvenlik kuralı geçerlidir:

```bash
git checkout prd
iflowkit sync push --to prd
```

---

## Sık sorulanlar

### “Feature branch’te neden pull yok?”

Çünkü `pull` “ortam branch’i = tenant gerçeği” yaklaşımına göre tasarlanmıştır. Feature branch’te pull yapmak, tenant state’ini work-in-progress branch’e taşımayı teşvik eder ve pratikte ekiplerde karışıklık yaratır.

Feature branch’te CPI’daki son hale ihtiyaç duyarsanız önerilen yol:

- `dev` branch’te `sync pull`
- sonra feature branch’inizi `dev`’den rebase/merge edin

### “Work branch push ettiğimde transport nerede?”

Tenant mapping DEV olduğu için:

- `.iflowkit/transports/dev/` altında kayıt oluşur

Transport kaydının `branch` alanı ise work branch adınızı taşır.
