# YAML 設定リファレンス

[English](README.md) · [README](../../README.ja.md) · [設定例](../../config.example.yaml)

設定の `version: 1` を対象とするリファレンスです。設定ファイルは `tamis check CONFIG`、`tamis preview CONFIG`、`tamis apply CONFIG` に明示的に渡します。設定ファイルの自動検索は行いません。

## 目次

- [設定全体の例](#設定全体の例)
- [最上位の項目](#最上位の項目)
- [Settings](#settings)
- [パス変数](#パス変数)
- [Location と glob パターン](#location-と-glob-パターン)
- [Rules と評価順](#rules-と評価順)
- [条件式](#条件式)
- [Actions](#actions)
- [テンプレートとフィルター](#テンプレートとフィルター)
- [検証](#検証)

## 設定全体の例

scan の前に入力元・保存先のパスを変更してください。この例は画像・動画をコピーし、元ファイルを残します。

```yaml
version: 1

settings:
  assume_timezone: Asia/Tokyo
  video:
    mvhd_timezone: local
  on_conflict: rename
  jobs: 0
  windows_names: false

vars:
  archive: ~/Media/Archive

rules:
  - name: archive-images
    location: ~/Media/Inbox
    recursive: true
    include: ["**/*.jpg", "**/*.JPG", "**/*.png", "**/*.mov", "**/*.mp4"]
    exclude: ["**/private"]
    when: 'kind == "image" && captured != nil'
    actions:
      - copy: '{{archive}}/Images/{{captured | date "2006/01"}}/'
        rename: '{{captured | date "20060102_150405"}}_{{basename}}{{ext}}'
    stop: true

  - name: archive-videos
    location: ~/Media/Inbox
    recursive: true
    include: ["**/*.jpg", "**/*.JPG", "**/*.png", "**/*.mov", "**/*.mp4"]
    exclude: ["**/private"]
    when: 'kind == "video" && captured != nil'
    actions:
      - copy: '{{archive}}/Videos/{{captured | date "2006/01"}}/'
```

## 最上位の項目

設定オブジェクトに未知の項目名を書くとエラーになります。`vars` と `shell.env` は、ユーザーがキーを定義する map です。

| 項目       | 型                   | 既定値・要件   | 意味                                 |
| ---------- | -------------------- | -------------- | ------------------------------------ |
| `version`  | 整数                 | 必須。`1` のみ | 設定のバージョン。                   |
| `settings` | オブジェクト         | 後述の既定値   | metadata 抽出とファイル操作の設定。  |
| `vars`     | 文字列の map         | 空             | テンプレートで参照する名前付きパス。 |
| `rules`    | オブジェクトのリスト | 空             | 入力ごとに記述順に評価する rule。    |

## Settings

| 項目                  | 型     | 既定値   | 意味                                                                                                                    |
| --------------------- | ------ | -------- | ----------------------------------------------------------------------------------------------------------------------- |
| `assume_timezone`     | 文字列 | `local`  | offset のない画像・PNG text・sidecar の日時に適用する timezone。`local`、`utc`、`Asia/Tokyo` などの IANA 名を指定可能。 |
| `video.mvhd_timezone` | 文字列 | `local`  | QuickTime の `mvhd` 作成日時の解釈。`local` または `utc` のみ。動画内の offset のない日時文字列にも適用。               |
| `on_conflict`         | 文字列 | `rename` | 宛先競合時の既定の方針。`rename`、`skip`、`fail` のいずれか。                                                           |
| `jobs`                | 整数   | CPU 数   | metadata 抽出の worker 数。省略・ゼロ・負数は CPU 数を使用。CLI の正の `--jobs` が優先。                                |
| `windows_names`       | 真偽値 | `false`  | ファイル名の正規化時に、Windows の予約名に該当する stem の先頭へ `_` を付加。                                           |

`local` はプロセスのローカル timezone です。`assume_timezone` は既に offset を持つ日時を変換せず、`video.mvhd_timezone` が使う timezone も変更しません。`mvhd_timezone: local` はデコードした年月日・時分秒をローカルの壁時計時刻として解釈し、`utc` は UTC として解釈します。`date` フィルターは日時が持つ timezone のまま整形します。

`windows_names` の対象は、大文字・小文字を区別しない `CON`、`PRN`、`AUX`、`NUL`、`COM1`〜`COM9`、`LPT1`〜`LPT9` で、拡張子付きの名前も含みます。Windows のファイル名制約をすべて処理するものではありません。ディレクトリ宛先に付加するファイル名に適用し、明示した宛先パスの全要素には適用しません。

どの rule にも一致しない入力は、撮影日時の有無にかかわらず skip し、そのまま残します。metadata 抽出失敗と判定された入力は、fallback で日時が得られていても skip し、そのまま残します。

## パス変数

```yaml
vars:
  archive: ~/Media/Archive
  exports: ./exports
```

各値はパス文字列です。先頭の `~/` を展開し、相対パスは**プロセスの作業ディレクトリ**を基準に解決します。設定ファイルのあるディレクトリは基準になりません。`rules[].location` も同じ扱いです。これらのパスでは `$HOME` などの環境変数は展開しません。

`copy`、`move`、`rename`、`shell.env` のテンプレートで、厳密に `{{archive}}` または `{{vars.archive}}` と書いて参照します。パス変数は metadata テンプレートの解析前に文字列置換します。この形式の内側の空白やパス変数へのフィルターは非対応です。`{{ archive }}` や `{{archive | lower}}` ではなく、`{{archive}}/{{model | slug}}/` と書いてください。metadata の項目名と重なる変数名には `{{vars.NAME}}` を使ってください。変数参照を入れ子にしないでください。

`vars` は汎用の文字列 map ではなく、`when` からも参照できません。ラベルや timezone offset などの固定文字列は、テンプレートや `shell.env` の値に直接書きます。

## Location と glob パターン

| 項目        | 型             | 既定値・要件       | 意味                                                                                     |
| ----------- | -------------- | ------------------ | ---------------------------------------------------------------------------------------- |
| `location`  | 文字列         | 必須。空でないこと | この rule の入力パス。                                                                   |
| `recursive` | 真偽値         | `false`            | サブディレクトリ内も scan。                                                              |
| `include`   | 文字列のリスト | 空: すべて対象     | 空でなければ、いずれかのパターンに一致するファイルだけを対象にする。                     |
| `exclude`   | 文字列のリスト | 空: 追加の除外なし | いずれかのパターンに一致する項目を除外。ディレクトリに一致すると、その配下もすべて除外。 |

通常ファイルのみを scan し、symlink や特殊ファイルは除外します。`exclude` は `include` より優先します。パターンはその rule の `location` からの相対パス全体に一致させ、区切り文字は `/`、大文字・小文字は区別します。名前が `.` で始まるファイル・ディレクトリは常に除外し、隠しディレクトリの配下も探索しません。`include` や `location`・`scan PATH` で明示しても対象になりません。この判定はパス名に基づき、OS 固有の hidden 属性は参照しません。

| パターン構文 | 意味                       | 例                                                                    |
| ------------ | -------------------------- | --------------------------------------------------------------------- |
| `*`          | 1つのパス要素内の0文字以上 | `*.jpg` はルート直下の JPEG ファイル名だけに一致。                    |
| `?`          | `/` 以外の1文字            | `image-?.png`                                                         |
| `**`         | `/` を含む0文字以上        | `archive/**` は `archive` 配下の項目に一致。                          |
| `**/`        | 0階層以上のディレクトリ    | `**/*.jpg` はルート直下とサブディレクトリ内の JPEG ファイル名に一致。 |

`[a-z]` などの文字クラス、`*.{jpg,png}` などの brace expansion、`!keep.jpg` などの否定は非対応で、これらの文字はそのまま照合します。拡張子や大文字・小文字の違いは個別のパターンで指定してください。`*` から始まるパターンは、YAML の alias と解釈されないよう引用符で囲みます。任意の階層の `private` ディレクトリを配下ごと除外するには `exclude: ["**/private"]` を使います。

`location`・`recursive`・`include`・`exclude` は各 rule に直接指定します。同じ探索設定は一度だけ探索し、探索範囲が重なるファイルも metadata 抽出は一度だけ行います。各ファイルは rule の記述順で評価し、`relpath` は評価中の rule の `location` 基準です。`scan` の JSON の `relpath` は、そのファイルを最初に見つけた rule の `location` 基準です。

設定内の全 rule を対象に探索・評価します。旧 `sources`・`source`・`--source` は受け付けません。scan JSON の `source_id` も廃止しています。

## Rules と評価順

| 項目      | 型                   | 既定値・要件         | 意味                                                                                                          |
| --------- | -------------------- | -------------------- | ------------------------------------------------------------------------------------------------------------- |
| `name`    | 文字列               | 必須。空でなく、一意 | 計画や説明に表示する名前。                                                                                    |
| `when`    | 文字列               | `"true"`             | 真偽値を返す Expr の式。                                                                                      |
| `capture` | 文字列               | なし                 | 拡張子を含むファイル名全体に対して照合する Go の正規表現。                                                    |
| `actions` | オブジェクトのリスト | 空                   | 記述順に計画する操作。空のリストなら操作なし。                                                                |
| `stop`    | 真偽値               | `false`              | この rule が一致したら、その入力に対する後続 rule の評価を終了。`move`・`trash` を含む rule では暗黙に true。 |

入力ごとに YAML の記述順に rule を確認します。最初に `location` と `recursive`・`include`・`exclude`、次に指定されていれば `capture`、最後に `when` を評価します。`capture` が一致しなければ、その rule を飛ばします。`(?P<year>\d{4})` などの名前付きグループは文字列の `capture.year` となり、その rule の `when` とテンプレートで使えます。capture の値は rule 間で共有しません。正規表現は `^`・`$` で固定しない限り部分一致です。

次の rule 断片は、`2001_02_sample.png` のような架空のファイル名から取り出した文字列で整理します。

```yaml
name: filename-date
location: ~/Media/Inbox
capture: '^(?P<year>\d{4})_(?P<month>\d{2})_.*\.png$'
when: 'kind == "image" && capture.year == "2001"'
actions:
  - copy: "~/Media/Archive/{{capture.year}}/{{capture.month}}/"
stop: true
```

capture グループは日時として解析せず、`captured` も更新しません。`actions` が空でも rule は一致したものとして扱い、`stop: true` と組み合わせて、該当入力を操作せずに残せます。どの rule にも一致しない入力は skip します。

`preview`・`apply` は `when` の参照項目から必要な metadata を決めます。同じファイルに対して探索範囲と `capture` が一致する rule の参照項目をまとめ、一度だけ抽出します。`when` の分岐や後続 rule の `stop` による絞り込みは抽出後に行うため、実際に成立する分岐より広い範囲を読むことがあります。

ファイル名や filesystem 属性だけを参照する場合、画像・動画 parser は呼びません。`kind`・`format`・`mime` には形式判定用の読み取りだけを行い、`date_added` は参照時のみ取得します。画像は必要なタグと metadata source を選択し、動画は日時と duration の必要性に応じて box を読みます。`raw` を参照すると広い範囲の metadata 抽出が必要になります。CR3/CRW/RW2 の parser はタグ別の抽出に対応していないため、必要な場合は一括解析しますが、`raw` 未参照なら全 metadata の JSON 変換は行いません。必要な項目を見つけるためのヘッダー・構造の読み取りは残ります。

要求した metadata の解析に失敗したファイルは引き続き skip します。要求していない metadata の破損は検出しないことがあります。`scan`・`explain` は診断用として従来どおり metadata 全体を抽出します。`$env` 経由の参照は非対応です。項目名を直接記述してください。

## 条件式

`when` はテンプレート言語ではなく Expr を使います。設定の読み込み時に compile し、結果は真偽値である必要があります。式の上限は AST の200ノードです。記述例:

```yaml
when: 'kind == "image" && captured != nil'
```

```yaml
when: 'ext in [".jpg", ".jpeg"] && age_hours >= 24'
```

```yaml
when: 'captured != nil && captured_source not in ["fs:birthtime", "fs:mtime"]'
```

アプリケーションが条件式に提供する変数は以下のとおりです。optional の値は `nil` の可能性があるため、比較やメンバー参照の前に確認してください。取得できない文字列は空、画像サイズはゼロです。変数名は `scan` の JSON の項目名とは異なります。

| 変数                    | 型                      | 意味                                                                                                                                        |
| ----------------------- | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `path`                  | 文字列                  | 入力の絶対パス。                                                                                                                            |
| `relpath`               | 文字列                  | その rule の `location` からの相対パス。区切り文字は `/`。                                                                                  |
| `basename`              | 文字列                  | 最後の拡張子を除いたファイル名。                                                                                                            |
| `ext`                   | 文字列                  | `.` を含む小文字の最後の拡張子。なければ空。                                                                                                |
| `dir`                   | 文字列                  | 直上のディレクトリ名。フルパスではない。                                                                                                    |
| `size`                  | 整数                    | ファイルサイズ。単位は byte。                                                                                                               |
| `kind`                  | 文字列                  | `image`、`video`、`other` のいずれか。                                                                                                      |
| `format`                | 文字列                  | `JPEG`、`TIFF`、`PNG`、`WEBP`、`HEIC`、`AVIF`、`DNG`、`CR2`、`CR3`、`CRW`、`NEF`、`PEF`、`ARW`、`RW2`、`MOV`、`MP4`、`UNKNOWN` のいずれか。 |
| `mime`                  | 文字列                  | MIME type。固有の対応付けがなければ `application/octet-stream`。                                                                            |
| `captured`              | optional の日時         | 採用された撮影日時。filesystem fallback の場合もある。                                                                                      |
| `captured_source`       | 文字列                  | 日時の出所。例: `fs:birthtime`、`fs:mtime`、`qt:mvhd.creation_time`。日時がなければ空。                                                     |
| `mtime`                 | 日時                    | filesystem の更新日時。                                                                                                                     |
| `ctime`                 | 日時                    | macOS で取得できれば filesystem の作成日時。それ以外は mtime。POSIX の inode 変更日時ではない。                                             |
| `date_added`            | optional の日時         | macOS で取得できる、ファイルが現在のディレクトリへ追加された日時。作成日時・mtime への fallback は行わない。                                |
| `age`                   | duration                | `now - ctime`。Go の duration としてナノ秒で表現。                                                                                          |
| `age_hours`             | 数値                    | `now - ctime` を小数の時間単位で表現。                                                                                                      |
| `now`                   | 日時                    | 計画内の入力で共有する評価時刻。                                                                                                            |
| `make`、`model`、`lens` | 文字列                  | カメラメーカー、カメラモデル、レンズモデル。                                                                                                |
| `iso`                   | optional の整数         | ISO 感度。                                                                                                                                  |
| `fnumber`               | optional の数値         | 絞りの F 値。                                                                                                                               |
| `focal`                 | optional の数値         | 焦点距離。単位は mm。                                                                                                                       |
| `width`、`height`       | 整数                    | 取得できた画像サイズ。単位は pixel。                                                                                                        |
| `gps`                   | optional のオブジェクト | 度単位の `gps.lat`・`gps.lon` と、optional の m 単位の `gps.alt`。                                                                          |
| `duration`              | optional の duration    | Go の duration としてナノ秒で表現する動画の長さ。                                                                                           |
| `raw`                   | map                     | provider 固有のデコード済み metadata。キーと値の型は parser・format に依存。                                                                |
| `capture`               | 文字列の map            | 現在の rule の正規表現の名前付きグループ。                                                                                                  |
| `seq`                   | 整数                    | 評価対象リスト内の1始まりの番号。CLI では重複を除いた絶対パス順。                                                                           |

`seq` は永続的な連番ではなく、設定内の rule や入力ファイルによって変わります。`explain` は1入力を `seq = 1` で評価します。日時の精度は format によって異なります。CR3/CRW/RW2 以外の画像では別の EXIF subsecond タグを日時に合成しないため、秒未満の値で名前の一意性を保証しないでください。

### フォルダへの追加日時

フォルダに追加されてから3日を超えたファイルを対象にする条件:

```yaml
when: "date_added != nil && now.Sub(date_added).Hours() > 72"
```

`date_added` は macOS の filesystem 属性 `ATTR_CMN_ADDEDTIME` を使い、ファイルが現在のディレクトリ内で作成された、または rename で追加された日時を表します。`ctime`・`mtime`・`age_hours` とは別の値です。organize が使う Spotlight の `kMDItemDateAdded` インデックスではなく filesystem から直接取得するため、インデックスの値と異なる場合があります。[Apple の getattrlist 仕様](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlist.2)

Linux、非対応の filesystem、値がない場合は `nil` です。リンクを削除済みのファイルや複数の hard link があるファイルも、所属ディレクトリが曖昧なため `nil` にします。他の日時で代用せず、撮影日時の fallback にも使いません。比較や `date` フィルターの前に `date_added != nil` を確認してください。`scan` の JSON には取得できた場合に出力し、`explain` には日時または `unavailable` を表示します。

上の条件は計画の固定された `now` を使い、厳密に72時間を超えた場合だけ一致します。作成日時・更新日時が古くても、新しく追加されたファイルは一致しません。`duration` は既に動画の長さを表す変数なので、`duration("72h")` ではなく、上記の日時のメソッドを使ってください。

## Actions

各 action オブジェクトは、空でない `copy`、空でない `move`、`trash: true`、`shell` オブジェクトから、必ず1種類だけ選びます。`rename` は copy・move のオプションであり、単独の action ではありません。

| 項目          | 型                 | 意味                                                                                                         |
| ------------- | ------------------ | ------------------------------------------------------------------------------------------------------------ |
| `copy`        | 文字列テンプレート | ファイルまたはディレクトリ宛先へコピー。                                                                     |
| `move`        | 文字列テンプレート | ファイルまたはディレクトリ宛先へ移動。rule の最後の action に置く。                                          |
| `trash`       | 真偽値             | `true` でゴミ箱の退避先ディレクトリへ移動。rule の最後の action に置く。                                     |
| `shell`       | オブジェクト       | copy より前に元の入力に対してコマンドを実行。詳細は後述。                                                    |
| `rename`      | 文字列テンプレート | 任意。`/` で終わる `copy`・`move` 宛先のファイル名。それ以外では無視。`trash` でも無視し、`shell` では禁止。 |
| `on_conflict` | 文字列             | 任意。`rename`、`skip`、`fail`。既定値は `settings.on_conflict`。`shell` では禁止。                          |

### 宛先とファイル名の変更

`/` で終わる宛先はディレクトリとして扱い、元のファイル名または `rename` の展開結果を付加します。末尾に `/` がなければ、既存ディレクトリを指していても完全なファイルパスとして扱います。`rename` は元の拡張子を自動で付けないため、必要なら `{{ext}}` を指定します。既定の元のファイル名は拡張子の大文字・小文字を保持しますが、`{{ext}}` は小文字です。

```yaml
actions:
  - copy: "~/Media/Archive/"
    rename: "{{basename | lower}}{{ext}}"
  - move: "~/Media/Processed/{{basename}}{{ext}}"
```

展開後の宛先は絶対パスである必要があります。`copy`・`move` の文字列先頭の `~/` は展開しますが、それ以外の相対宛先はエラーになります。絶対パスまたはパス変数をテンプレートの起点にしてください。必要な宛先の親ディレクトリは実行時に作成します。

ディレクトリ宛先に付加するファイル名は Unicode NFC に正規化します。`/`・`\`・NUL を `-` に置換し、可能な限り拡張子を保ちながら UTF-8 で255 byte 以内に短縮します。`rename` の結果が空文字・`.`・`..` になる場合はエラーです。明示的な完全ファイルパスには、このファイル名正規化を適用しません。

### 競合時の方針と action の順序

| 方針     | 動作                                                                                      |
| -------- | ----------------------------------------------------------------------------------------- |
| `rename` | `name-2.jpg`、`name-3.jpg` のような空いている名前を選択。同じ計画で予約済みの宛先も考慮。 |
| `skip`   | 競合する操作を省略。既存ファイルの内容は比較しない。                                      |
| `fail`   | 宛先が使用済みなら計画をエラーにする。                                                    |

CLI の `--force` を指定しても、既存の宛先は上書きしません。実行中に新たな競合を検出した場合、検証済みの計画から別の名前を選び直さずに停止します。

同じ入力のすべての `shell` は、一致する複数 rule にまたがる場合も、最初の copy より前に置く必要があります。任意の個数の copy の後に、最後の move または trash を最大1個置けます。move・trash は明示的に `stop: false` としても後続 rule の評価を終了します。copy のみ、または shell のみの rule は、`stop: true` がなければ後続 rule も評価します。

同じ入力の copy が1つでも skip された場合、残りの copy は続けますが、最後の move・trash は skip して元ファイルを残します。複数 rule にまたがる場合も同じです。エラー時は実行を停止し、完了済みのコピーや shell による変更は巻き戻しません。

`copy` は macOS で clone を試し、非対応や別ボリュームの場合は通常コピーに切り替えます。Linux は通常コピーです。権限・mtime・macOS の拡張属性を保持します。容量検証は clone で節約できる可能性があってもファイル全体のサイズで見積もります。

### Trash

macOS と Linux の `trash` は、[github.com/mono0x/trash](https://github.com/mono0x/trash) を使い、外部コマンドや CGO なしで OS 標準のゴミ箱へ移動します。macOS は Foundation、Linux は実行時に `libgio-2.0.so.0` が必要な GIO を使います。Linux のゴミ箱の配置と復元用情報は GIO が管理します。移動先と同名ファイルの処理は OS に任せ、`preview` は `system Trash` と表示し、JSON では `dest` を省略します。ネイティブライブラリとゴミ箱の利用可否は実行時に確認し、移動先の容量は事前確認しません。失敗すると処理を停止し、完了済みのコピーは残します。代わりに直接削除することはありません。Finder の「元に戻す」は保証しません。`preview CONFIG --trash-dir PATH`・`apply CONFIG --trash-dir PATH` で明示的な退避先へ移動する場合は、宛先を実行前に表示・検証します。この退避先にはデスクトップのゴミ箱での復元用情報を記録しません。YAML に `trash_dir` 設定はありません。

### Shell

| 項目        | 型           | 既定値・要件                   | 意味                                                                                                                            |
| ----------- | ------------ | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------- |
| `shell.run` | 文字列       | 必須。空白のみは不可、NUL 禁止 | `/bin/sh -eu -c` でそのまま実行するスクリプト。テンプレート展開なし。                                                           |
| `shell.env` | 文字列の map | 空                             | テンプレート言語で展開する環境変数の値。変数名は `^[A-Za-z_][A-Za-z0-9_]*$` に一致する必要があり、値に NUL を含めることは禁止。 |

`TAMIS_FILE` は予約済みで、元の入力ファイルの絶対パスを含みます。`env` では設定できません。スクリプト内の環境変数の展開は引用符で囲んでください。次の action 断片は、インストール済みのスクリプトへ metadata の値を渡す例です。

```yaml
when: 'model matches ".*"'
actions:
  - shell:
      run: 'sh ./prepare-media.sh "$TAMIS_FILE" "$CAMERA"'
      env:
        CAMERA: '{{model | default "unknown"}}'
  - copy: "~/Media/Archive/"
```

コマンドはプロセスの作業ディレクトリと環境変数を引き継ぎ、`env` の項目が既存の環境変数を上書きします。stdin は渡しません。stdout は破棄し、失敗時のエラーには stderr の先頭16 KiB までを含めます。timeout は5分で、YAML では変更できません。バックグラウンド処理は非対応で、終了・キャンセル時に process group を終了します。

shell はユーザー権限で sandbox なしに動作し、元ファイルを直接変更できるため、信頼できる設定だけを実行してください。`preview`・`explain` はコマンドと展開済みの環境変数を表示し、実行しません。成功後の入力は引き続き通常ファイルである必要があり、後続操作のために同一性・サイズ・mtime を更新します。shell 後に metadata・条件・宛先は再評価しません。コマンド失敗時は変更を巻き戻さずに停止し、後続 copy が skip された場合も加工済みの元ファイルを残します。

## テンプレートとフィルター

テンプレートは `copy`・`move`・`rename`・`shell.env` の値に適用します。小さな `{{ variable | filter arguments }}` 言語を使い、Expr や Go の `text/template` ではありません。任意の式・算術演算・条件分岐・関数呼び出しは非対応です。

アクションの metadata 変数 `captured`・`mtime`・`ctime`・`date_added`・`make`・`model`・`lens` は、同じ rule の `when` で参照した項目だけ使えます。宛先・`rename`・`shell.env` のすべてで検証し、未参照なら設定読み込み時にエラーにします。他の rule の参照では代用できません。`path`・`relpath`・`basename`・`ext`・`dir`・`seq` は基本変数として常に使え、`capture.NAME` はその rule の `capture` で取得します。単に文字列として `"model"` と書いても参照にはなりません。

テンプレートで使える metadata 変数は `path`、`relpath`、`basename`、`ext`、`dir`、`captured`、`mtime`、`ctime`、`date_added`、`make`、`model`、`lens`、`seq`、`capture.NAME` のみです。意味は条件式の表を参照してください。`kind`・`size`・`iso`・`raw`・`now`・`captured_source` など、それ以外の条件式の変数はテンプレートでは使えません。パス変数は前述の文字列置換で扱います。

フィルターは左から右に適用します。空白を含む引数は二重引用符で囲み、YAML の値全体を単一引用符で囲むと書きやすくなります。テンプレートの parser は、引数内の引用符のエスケープや文字としての `|` に対応しません。存在しない capture グループは空文字です。`captured` がなければフィルターなしでは空文字になりますが、`date` の適用はエラーになります。

| フィルター        | 意味                                                                                                         | 例                                       |
| ----------------- | ------------------------------------------------------------------------------------------------------------ | ---------------------------------------- |
| `date "LAYOUT"`   | Go の date layout で日時を整形。入力は日時が必須。                                                           | `{{captured \| date "2006/01/02"}}`      |
| `slug`            | NFKD 正規化して小文字化。Unicode の文字・数字を残し、それ以外の連続部分を `-` に置換、両端の `-` を除去。    | `{{model \| slug}}`                      |
| `lower`           | 小文字化。                                                                                                   | `{{basename \| lower}}`                  |
| `upper`           | 大文字化。                                                                                                   | `{{capture.label \| upper}}`             |
| `pad WIDTH`       | 指定した最小幅まで左側をゼロで埋める。短縮はしない。                                                         | `{{seq \| pad 4}}`                       |
| `default "TEXT"`  | `nil` または空文字の場合に置換。ゼロは対象外。                                                               | `{{model \| default "unknown" \| slug}}` |
| `trunc LENGTH`    | Unicode code point 単位で指定長まで残す。長さは非負。                                                        | `{{basename \| trunc 40}}`               |
| `slice START END` | Unicode code point 単位で0始まりの START から END の直前までを取り出す。`0 <= START <= END <= 長さ` が必要。 | `{{basename \| slice 0 4}}`              |

`slug` は非ラテン文字も保持し、ASCII への翻字は行いません。

Go の date layout は基準日時 `2006-01-02 15:04:05` を使います。`YYYY-MM-DD` や `strftime` の指定子は使いません。

| Layout            | 2001-02-03 04:05:06 の出力 |
| ----------------- | -------------------------- |
| `2006/01`         | `2001/02`                  |
| `2006-01-02`      | `2001-02-03`               |
| `20060102_150405` | `20010203_040506`          |

宛先テンプレートの展開値は、原則として1つのパス要素に収まる必要があります。`/`・`\` を含む値には `slug` を使ってください。`path`、`relpath`、`captured`、`mtime`、`ctime`、`date_added` だけは `/` を含む複数階層に展開でき、`2006/01` のような layout を使えます。すべての展開値で NUL・`\`・`.` または `..` に等しいパス要素は禁止します。`rename` と `shell.env` はテキストとして展開し、`rename` の結果には前述のファイル名正規化を適用します。

## 検証

```sh
tamis check ./organize.yaml
tamis preview ./organize.yaml
tamis explain ./organize.yaml /absolute/path/to/input.jpg
```

`check` は YAML を読み込み、未知の項目や不正な action 構造を拒否し、timezone・競合方針を検証して、条件式と capture の正規表現を compile します。入力の scan、metadata に依存するテンプレートの展開、宛先の存在・権限・容量の検証は行いません。設定の検証に成功しても、評価時や操作の検証時に失敗する場合があります。

`preview` は全 rule の入力 を scan・評価し、ファイルの変更や shell の実行なしに計画全体を検証します。`explain` は1入力の metadata 抽出と rule 評価を行いますが、`preview` のような計画全体の filesystem 検証は行いません。`apply CONFIG` は再 scan・検証を経て、計画された操作を実行します。
