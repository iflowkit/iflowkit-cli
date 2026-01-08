# Transport Kayıtları (`.iflowkit/transports/`)

`iflowkit sync` modülü her önemli operasyonu (init/pull/push/deliver) bir **transport** olarak kayıt altına alır.

Transport kayıtları iki ana amaç için vardır:

1. **Audit / izlenebilirlik**: “Ne değişti? Hangi branch’teydi? Hangi tenant’a uygulandı? Hangi commit’ler vardı?”
2. **Retry / kaldığı yerden devam**: CPI tarafında hata olursa, bir sonraki çalıştırmada kaldığı yerden devam edebilmek.

---

## Klasör yapısı

Transport dosyaları repo içinde şurada tutulur:

```text
.iflowkit/transports/<tenant>/
  index.json
  <transportId>.transport.json
```

Buradaki `<tenant>` değeri **branch değil**, **hedef tenant environment**’tır:

- `dev`
- `qas`
- `prd`

> Örnek: `feature/*` branch’lerinde `sync push` çalıştırırsanız tenant mapping DEV olduğu için kayıtlar `.../transports/dev/` altında oluşur.

---

## Transport ID formatı

Kodun ürettiği `transportId` formatı:

- `YYYYMMDDTHHMMSSmmmZ` (UTC)
  - `mmm`: milisaniye (3 basamak)

Örnek:

- `20260106T101530123Z`

Ayrıca kayıt içinde `createdAt` alanı vardır:

- RFC3339 (UTC, saniye hassasiyeti)
  - ör: `2026-01-06T10:15:30Z`

---

## `index.json`

`index.json` her tenant için transport’ların düz (flat) listesini tutar:

- hızlı “en son transport” bulma
- “pending var mı?” bakabilme

Şema (özet):

```json
{
  "schemaVersion": 1,
  "items": [
    {
      "seq": 1,
      "transportId": "20260106T101530123Z",
      "transportType": "push",
      "transportStatus": "pending",
      "createdAt": "2026-01-06T10:15:30Z"
    }
  ]
}
```

- `seq` sadece yerel sıralama içindir.
- `transportStatus`: `pending` veya `completed`.

---

## `<transportId>.transport.json`

Her transport için ayrıntılı kayıt dosyasıdır.

### Alanlar ve anlamları

| Alan | Açıklama |
|---|---|
| `schemaVersion` | Şema versiyonu (şu an 1) |
| `transportId` | Transport kimliği |
| `transportType` | `init` \| `pull` \| `push` \| `deliver` |
| `packageId` | CPI IntegrationPackage id |
| `branch` | Operasyonun çalıştığı branch (örn. `dev`, `feature/x`, `prd`) |
| `createdAt` | RFC3339 zaman |
| `gitCommits` | Bu transport sırasında git’e “push edilmiş veya planlanmış” commit SHA listesi |
| `gitUserName` / `gitUserEmail` | Git identity (bulunursa) |
| `objects` | Bu transport’ta “changed” kabul edilen objeler (kind + id) |
| `deletedObjects` | Bu transport’ta silinen objeler (kind + id) |
| `transportStatus` | `pending` veya `completed` |
| `error` | Son hatanın string mesajı (pending ise dolu olabilir) |
| `uploadRemaining` | CPI’ya henüz upload edilmemiş artefact listesi (retry state) |
| `deleteRemaining` | CPI’dan henüz silinmemiş artefact listesi (retry state) |
| `deployRemaining` | CPI’da henüz deploy edilmemiş artefact listesi (retry state) |

### `objects` vs `uploadRemaining`

- `objects`: “bu transport bu objeleri değiştirdi” diye raporlanan set.
- `uploadRemaining`: CPI işlerinde retry için kullanılan gerçek “iş kuyruğu”.

Normalde `sync push` / `sync deliver`:

1) `deleteRemaining` (varsa) işler
2) `uploadRemaining` işler
3) `deployRemaining` işler

Her adım tamamlandıkça ilgili `...Remaining` listeleri küçülür ve kayıt tekrar yazılır.

---

## Silinen objeler (`deletedObjects`)

Bu alan iki farklı senaryoyu kapsar:

- **Pull**: CPI’da silinmiş objeler repo’dan kaldırılır. `deletedObjects` burada “CPI → Git deletion” anlamına gelir.
- **Push/Deliver**: Repo’da silinen objeler CPI’dan silinir. `deletedObjects` burada “Git → CPI deletion” anlamına gelir.

> Not: Silme işlemi her kind için desteklenmeyebilir. Örn. `CustomTags` için delete endpoint’i yoksa, kayıt altında görünse bile CPI delete adımı “no-op” olabilir.

---

## Retry (kaldığı yerden devam) nasıl çalışır?

### Ne zaman “pending” olur?

CPI tarafındaki herhangi bir hata (auth, network, 4xx/5xx) olduğunda:

- transport `pending` kalır
- `error` alanına hata yazılır
- o ana kadar tamamlanan adımlar kayıt içinde kalır (ör. bazı upload’lar yapılmış, bazıları `uploadRemaining`’de duruyor olabilir)

### Devam etmek için ne yapmalıyım?

En basit senaryo:

- Aynı branch’te aynı komutu tekrar çalıştırın.

Örnek:

```bash
# bir önceki push CPI upload sırasında patladıysa
iflowkit sync push

# deliver yarıda kaldıysa
iflowkit sync deliver --to prd
```

`sync push` / `sync deliver` çalışırken hedef tenant’a göre ilgili store’dan şunu arar:

- “En son **completed olmayan** kayıt”
- `packageId` eşleşmeli
- `branch` eşleşmeli
- `transportType` eşleşmeli

Bulursa, “yeni bir transport oluşturmak” yerine bu kaydı **resume** eder.

### Pending kaydını nasıl bulurum?

- `cat .iflowkit/transports/<env>/index.json`
- veya `ls .iflowkit/transports/<env>/*.transport.json | tail -n 5`

Ayrıca deploy durumlarını görmek için:

```bash
iflowkit sync deploy status --env <dev|qas|prd>
iflowkit sync deploy status --env <env> --transport <transportId>
```

---

## Git commit ve tag’ler

Sync, her transport’ı iki commit türüyle ilişkilendirir:

- **contents commit**: yalnızca `IntegrationPackage/` (veya `baseFolder`) değişiklikleri
- **logs commit**: `.iflowkit/` ve transport kayıtları

Commit mesaj formatı:

```text
<transportId> <transportType> <commitType> [ekMesaj]

# örnek
20260106T101530123Z push contents Fix mapping
20260106T101530123Z push logs Fix mapping
```

Tag formatı:

```text
<transportId>_<branch>

# örnek
20260106T101530123Z_dev
20260106T101530123Z_prd
```

Tag üretimi:

- `sync push`: sadece **environment branch**’lerde ve yalnızca “tam başarı” sonrası
- `sync deliver`: target branch’te “tam başarı” sonrası
- `sync init`: tag üretmez (ama deliver sırasında eksik env branch bootstrapping yapılırsa o bootstrap adımı tag üretebilir)

