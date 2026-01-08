# `.iflowkit/ignore` (Ignore Kuralları)

`iflowkit sync` modülü, bazı dosyaların CPI export’ları arasında **doğal olarak değiştiğini** ama bu değişikliklerin “fonksiyonel” olmadığını varsayar (örn. timestamp’li metadata).

Bu nedenle sync, artefact farkı hesaplarında ve bazı güvenlik kontrollerinde **ignore pattern’leri** uygular.

> Önemli: Ignore dosyası, git’in `.gitignore`’u gibi “versiyon kontrolünden hariç tutma” amacıyla değil; **hangi değişikliklerin senkronizasyonu tetikleyeceğini** belirlemek için kullanılır.

---

## Dosya konumu

Sync repo içinde ignore dosyası şuradadır:

- `/.iflowkit/ignore`

`sync init` bu dosyayı yoksa otomatik oluşturur.

---

## Varsayılan ignore’lar (built-in)

Aşağıdaki pattern’ler, `.iflowkit/ignore` dosyası hiç olmasa bile otomatik uygulanır:

```text
IntegrationPackage/**/metainfo.prop
IntegrationPackage/**/src/main/resources/parameters.prop
```

Bu iki dosya CPI export’larında çoğu tenant’ta “volatile” olduğu için varsayılan olarak ignore edilir.

> Senin örneğin olan `IntegrationPackage/` altında yer alan tüm `metainfo.prop` dosyalarını ignore etmek için ekstra bir şey yapmana gerek yok; bu zaten built-in default’tur.

---

## Pattern dili (desteklenen söz dizimi)

Ignore dili, **repo-relative path**’leri (repo kökünden itibaren) eşleştiren basit bir glob’dur.

### Desteklenen wildcard’lar

- `*` : `/` hariç her şeyi eşleştirir
- `?` : `/` hariç tek karakter eşleştirir
- `**` : path segment’leri dahil her şeyi eşleştirir

### Yorum ve boş satırlar

- `#` ile başlayan satırlar yorumdur
- Boş satırlar yok sayılır

### “Slash yoksa her yerde ara” kuralı

Bir pattern içinde `/` yoksa, sync onu otomatik olarak “her yerde ara” olarak yorumlar:

- `metainfo.prop`
  - efektif olarak: `**/metainfo.prop`

Bu sayede basit ignore kuralları yazmak kolaylaşır.

### Path separator kuralı

- Ignore dosyasında her zaman `/` (slash) kullanın.
- Windows’ta bile match işlemi repo path’lerini `/`’ye normalize eder.

---

## Ignore neyi etkiler?

### 1) `sync push`: hangi artefact “değişti” sayılır?

`sync push` şunu yapar:

1. Git diff ile değişen dosya path’lerini çıkarır
2. `.iflowkit/ignore` uygular
3. Kalan path’lerden **artefact set’i** üretir

Sonuç:

- Eğer bir artefact altında *yalnızca* ignore edilen dosyalar değiştiyse, o artefact **CPI’ya upload/deploy edilmeyebilir**.
- Ama bu dosyalar yine git commit’ine girebilir (git ignore değildir).

### 2) `sync compare`: fark listesini filtreler

`sync compare --to <env>` sadece ignore sonrası kalan farkları listeler.

### 3) `sync deliver`: tenant vs target branch eşitlik kontrolü

`sync deliver` çalışmadan önce güvenlik amacıyla:

- hedef tenant’ın export’u ile hedef branch içeriğini karşılaştırır
- bu diff üzerinde ignore uygular
- ignore sonrası hâlâ fark varsa deliver **fail** eder

Bu yüzden ignore dosyası “promote öncesi drift kontrolü”nde kritiktir.

---

## Örnekler

### Örnek 1: Tüm `metainfo.prop` dosyalarını ignore et

Zaten default olarak var; yine de açıkça yazmak istersen:

```text
IntegrationPackage/**/metainfo.prop
```

Daha kısa bir yazım (slash yok, her yerde arar):

```text
metainfo.prop
```

### Örnek 2: Tüm `parameters.prop` dosyalarını ignore et

```text
parameters.prop
# veya
IntegrationPackage/**/src/main/resources/parameters.prop
```

### Örnek 3: Bir iFlow içinde belirli bir dosyayı ignore et

```text
IntegrationPackage/iFlows/MyFlow/META-INF/MANIFEST.MF
```

### Örnek 4: Tüm iFlow’larda MANIFEST ignore

```text
IntegrationPackage/iFlows/**/META-INF/MANIFEST.MF
```

### Örnek 5: Bir klasör ağacını ignore et

Örn. bazı export’larda dev tenant’ta oluşan cache dosyalarını ignore etmek:

```text
IntegrationPackage/**/cache/**
```

---

## Sık yapılan hatalar

### 1) Git ignore ile karıştırmak

`.iflowkit/ignore` dosyası:

- dosyaları git’ten gizlemez
- sadece “diff → artefact set” dönüşümünde filtre görevi görür

Eğer gerçekten git’e dahil olmasını istemediğiniz dosyalar varsa, `.gitignore` kullanın.

### 2) Windows path yazmak

Şu yanlıştır:

```text
IntegrationPackage\\**\\metainfo.prop
```

Doğrusu:

```text
IntegrationPackage/**/metainfo.prop
```

### 3) Negation (`!pattern`) beklemek

Bu ignore dilinde **negation yoktur**.

Bir şeyi dahil etmek için “üstte ignore edip altta `!` ile geri alma” gibi davranış desteklenmez.

---

## Debug önerileri

Ignore davranışını doğrulamak için:

1) Değişiklik yaptığınız dosya path’lerini görün:

```bash
git status --porcelain
```

2) `sync compare` ile ignore sonrası farkı kontrol edin:

```bash
iflowkit sync compare --to prd
```

3) Eğer bir artefact’in CPI güncellemesi tetiklenmiyorsa, o artefact içindeki değişikliklerin ignore kapsamına girip girmediğini kontrol edin.
