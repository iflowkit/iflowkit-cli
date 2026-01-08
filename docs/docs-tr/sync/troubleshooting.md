# Troubleshooting (Sync)

Bu doküman, `iflowkit sync` kullanırken karşılaşabileceğiniz en yaygın problemleri ve pratik çözüm adımlarını listeler.

> Not: Komutlar hem stdout’a özet yazar hem de ayrıntılı log’u `os.UserConfigDir()/iflowkit/logs/` altına atar. İlk adım olarak `--log-level debug` ile çalıştırmak faydalıdır.

---

## 1) “not inside a sync repository: .iflowkit directory not found”

**Neden:** Komutu `sync init` ile oluşturulmuş bir repo içinde çalıştırmıyorsunuz.

**Çözüm:**

1. Repo kökünü bulun (`.iflowkit/` klasörü olan yer).
2. O dizine `cd` edin.

Örnek:

```bash
cd com.example.cpi.pkg
iflowkit sync pull
```

---

## 2) “missing metadata file: expected .../.iflowkit/package.json”

**Neden:** Repo içinde `.iflowkit` var, ama `package.json` yok veya silinmiş.

**Çözüm:**

- Bu repo büyük olasılıkla “sync repo” değildir.
- Eğer yanlışlıkla silindiyse, git üzerinden geri alın:

```bash
git checkout -- .iflowkit/package.json
```

---

## 3) “<ENV> tenant not found for profile ...; import it with `iflowkit tenant import ...`”

**Neden:** Hedef environment için CPI service key import edilmemiş.

**Çözüm:** Tenant key’i import edin:

```bash
iflowkit tenant import --env dev --file ./service-key-dev.json
iflowkit tenant import --env qas --file ./service-key-qas.json
iflowkit tenant import --env prd --file ./service-key-prd.json
```

> `dev` her zaman gerekir. `qas` sadece `cpiTenantLevels=3` ise gerekir.

---

## 4) PRD’e çalışmıyor: “confirmation required: pass --to prd”

**Neden:** PRD’e yanlışlıkla işlem yapılmasını önlemek için `--to prd` zorunlu.

**Çözüm:**

```bash
git checkout prd
iflowkit sync push --to prd
# veya
iflowkit sync pull --to prd
# veya (deliver’de hedef zaten prd olduğu için aynı zamanda onay)
iflowkit sync deliver --to prd
```

---

## 5) `sync pull` diverged hatası: “local branch diverged from origin/<branch> (ahead=..., behind=...)”

**Neden:** Lokal branch ile remote branch birbirinden ayrı commit’ler içeriyor.

**Çözüm seçenekleri:**

- Eğer local commit’lerinizi koruyacaksanız: rebase/merge ile düzeltin.
- Eğer remote’u referans alacaksanız: hard reset.

Örnek (dikkatli kullanın):

```bash
git fetch origin
git checkout dev
git reset --hard origin/dev
```

> `sync pull` bu durumu otomatik çözmez; çünkü yanlış merge/rebase risklidir.

---

## 6) `sync deliver` çalışmıyor: “working tree is not clean ...”

**Neden:** Deliver merge işlemi yaptığı için komut, çalışma ağacının temiz olmasını ister.

**Çözüm:**

- Değişiklikleri commit edin veya stash’leyin:

```bash
git status
# gerekiyorsa
 git stash push -u
```

Sonra:

```bash
iflowkit sync deliver --to qas
```

---

## 7) `sync pull` stash yaptı: “Stashed local changes (...)”

**Neden:** `sync pull`, CPI’dan export alırken `IntegrationPackage/` klasörünü silip yeniden yazdığı için local değişikliklerinizi güvenli şekilde saklamak amacıyla stash kullanır.

**Ne yapmalı?**

- Stash listesini görün:

```bash
git stash list
```

- Geri almak için:

```bash
git stash pop
```

> `sync pull` sadece `.iflowkit/transports/` altındaki değişiklikleri “non-blocking” kabul eder; diğer değişiklikleri stash’ler.

---

## 8) CPI upload/deploy sırasında hata oldu (transport “pending” kaldı)

**Belirti:** Komut hata ile biter ve `.iflowkit/transports/<env>/...transport.json` içinde `transportStatus: "pending"` kalır.

**Neden:** CPI tarafında ağ kesintisi, authentication, lock, geçici 5xx vb. olabilir.

**Çözüm:**

1. Sorunu düzeltin (service key, ağ, CPI availability...).
2. Aynı komutu tekrar çalıştırın.

Örnek:

```bash
# push sırasında kaldıysa
iflowkit sync push

# deliver sırasında kaldıysa
iflowkit sync deliver --to prd
```

`sync`, en son pending kaydı bulur ve `uploadRemaining / deleteRemaining / deployRemaining` listelerine bakarak kaldığı yerden devam eder.

> Manuel “reset” gerekmez. Pending kaydı silmek ancak “en son çare” olmalıdır.

---

## 9) `sync compare` “target branch origin/<to> does not exist”

**Neden:** `origin/qas` veya `origin/prd` branch’i remote’da yok.

**Çözüm:**

- 3-tenant modelde `qas` branch’i genelde ilk `deliver --to qas` veya `deliver --to prd` sırasında otomatik bootstrapped olur.
- Alternatif olarak, ilgili branch’i oluşturup push edebilirsiniz.

Örnek (deliver ile otomatik bootstrap):

```bash
iflowkit sync deliver --to qas
```

Bu komut gerekiyorsa tenant’tan export alıp branch’i oluşturur.

---

## 10) “git not found” / git komutları çalışmıyor

**Neden:** `git` binary’si PATH’te yok.

**Çözüm:**

- Git’i kurun.
- Terminali yeniden açın.
- `git --version` ile doğrulayın.

---

## 11) Deploy durumu görmek istiyorum

Transport içindeki objelerin CPI runtime status bilgisi:

```bash
# en son dev transport
iflowkit sync deploy status --env dev

# belirli bir transport
iflowkit sync deploy status --env prd --transport 20260106T101530123Z
```

Çıktı sütunları:

- `KIND`: iFlows / Scripts / ...
- `NAME`: artefact id
- `STATUS`: CPI runtime status (veya NOT_FOUND / ERROR)
- `DEPLOYED_AT`: CPI’nin raporladığı deploy zamanı
