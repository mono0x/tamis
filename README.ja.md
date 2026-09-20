# Tamis

[English](README.md)

Tamis は、metadata に基づく YAML rule で画像・動画を整理する高速な CLI です。rename、copy、move、trash に対応し、1つの入力を複数の宛先へ振り分けられます。

`preview` で操作を確認し、`apply` で直接実行します。中間の計画ファイルは不要です。単一 binary として動作し、純 Go の metadata parser を使い、ExifTool のインストールは不要です。外部コマンドは明示的に設定した `shell` action でのみ実行します。

## インストール

[mise](https://mise.jdx.dev/dev-tools/backends/github.html) を使う場合は、利用する `mise.toml` に次を追加してください:

```toml
[tools]
"github:mono0x/tamis" = "latest"
```

インストールしてバージョンを確認します:

```sh
mise install
mise exec -- tamis --version
```

[GitHub Releases](https://github.com/mono0x/tamis/releases) から macOS / Linux と architecture（`amd64` / `arm64`）に合う archive をダウンロードし、展開した `tamis` を `PATH` に配置してください。各 Release に SHA-256 checksum を記載した `checksums.txt` が付属します。`tamis --version` でバージョンを確認できます。

source からインストールする場合は Go 1.27.1 以降が必要です:

```sh
go install github.com/mono0x/tamis/cmd/tamis@latest
```

## クイックスタート

```sh
cp config.example.yaml organize.yaml

tamis check ./organize.yaml
tamis scan --config ./organize.yaml
tamis preview ./organize.yaml
tamis apply ./organize.yaml
```

scan の前に設定の入力元・保存先 path を編集してください。`preview` は入力元を scan し、予定する操作を検証・表示します。ファイルは変更しません。`apply` は再度 scan して全操作を検証し、確認を挟まず実行します。実行内容にはその時点のファイルの状態が反映されます。既存の宛先は上書きせず、設定で `rename`、`skip`、`fail` のいずれかを選択します。 `--force` でも宛先の競合は無視しません。実行中に新たな競合が発生した場合は、宛先を上書きせずに停止します。

`preview`・`apply` は `when` で参照した metadata だけを必要に応じて抽出します。アクションで使う日時・カメラ情報なども、同じ rule の `when` で参照が必要です。`scan`・`explain` は診断用に metadata 全体を抽出します。

scan は通常ファイルのみを対象とし、名前が `.` で始まるファイル・ディレクトリとその配下、symlink、FIFO などの特殊ファイルを除外します。実行前に入力の同一性・サイズ・更新日時を scan 時の状態と照合します。入力の変更や新たな rename 競合を検出した場合は停止し、表示した宛先を再度 rename することはありません。

rule に `include` パターンを指定すると、一致するファイルが存在し得ないディレクトリは探索しません。たとえば `/Volumes` に対する `Untitled*/PRIVATE/M4ROOT/CLIP/*` は、`Untitled*` に一致するボリューム内の指定階層だけを探索します。`**` を含むパターンでは、そのワイルドカード以降の再帰探索を維持します。

入力元の一部が読めない場合、`scan` は読めたレコードを出力したうえでエラー終了します。`preview`・`apply` は不完全な scan を受け付けません。クロスデバイスの move は、コピーしたファイルと宛先ディレクトリを同期してから元ファイルを削除します。macOS では拡張属性を保持できない場合に copy を停止し、属性を失ったまま move を完了することを防ぎます。

## 設定

各項目・既定値・条件式の変数・テンプレートフィルターは [YAML 設定リファレンス](docs/configuration/README.ja.md) を参照してください。

完全な例は [config.example.yaml](config.example.yaml) を参照してください。rule の条件には [Expr](https://expr-lang.org/) を、path には `{{ ... }}` template を使います。

```yaml
version: 1

settings:
  on_conflict: rename

rules:
  - name: archive-images
    location: ~/Media/Inbox
    recursive: true
    when: 'kind == "image" && captured != nil'
    actions:
      - copy: '~/Media/Archive/{{captured | date "2006/01"}}/'
        rename: '{{captured | date "20060102_150405"}}_{{basename}}{{ext}}'
```

action は `shell`、`copy`、`move`、`trash` です。`shell` は、複数 rule にまたがる場合も、その入力の最初の copy より前に置きます。入力ファイルごとに `copy` を任意の個数並べ、その後に `move` または `trash` を最大1個指定できます。`move`・`trash` は rule の最後の action に置き、その入力に対する rule 評価を自動終了します。後続の rule は評価しません。copy のみの rule は、`stop: true` を指定しない限り後続 rule も評価します。template filter は `date`、`slug`、`lower`、`upper`、`pad`、`default`、`trunc`、`slice` を利用できます。

最後の `move`・`trash` は、前の rule も含め、その入力に対する copy がすべて成功した場合だけ実行します。既存宛先との競合で copy が1つでも skip された場合、残りの copy は続けますが、最後の move/trash は省略して元ファイルを残します。既存ファイルとの内容比較は行いません。`preview` では既存のコピー先による省略を警告し、表示する move/trash は copy 成功を条件とする操作として扱います。`apply` は省略理由を stderr に表示し、JSON 出力では result の `skipped` 配列にも記録します。エラー時は実行を停止し、成功済みのコピーは巻き戻さず残します。

`copy` は macOS で自動的に clone を試し、非対応や別ボリュームの場合は通常コピーに切り替えます。Linux は通常コピーを使います。どちらも権限・拡張属性（macOS）・mtime を保存し、一時ファイルを同期してから既存の宛先を上書きせずに配置します。それ以外の clone エラーでは停止します。容量検証では、clone で節約できる場合も通常コピーに必要な容量を見積もります。`clone`・`hardlink`・`symlink` action は廃止しました。独立したファイルを作るには `copy` を使ってください。

宛先 template の `model`・`make` などの metadata は単一の path 要素として扱います。区切り文字を含む場合は `{{model | slug}}` を使ってください。`path`・`relpath`・date layout は複数のディレクトリを表現できますが、展開した値に `.`・`..` 要素を含めることはできません。

macOS と Linux の `trash` は、[github.com/mono0x/trash](https://github.com/mono0x/trash) を使い、外部コマンドや CGO なしで OS 標準のゴミ箱へ移動します。macOS は Foundation、Linux は実行時に `libgio-2.0.so.0` が必要な GIO を使います。Linux のゴミ箱の配置と復元用情報は GIO が管理します。移動先と同名ファイルの処理は OS に任せ、`preview` は `system Trash` と表示し、JSON では `dest` を省略します。ネイティブライブラリとゴミ箱の利用可否は実行時に確認し、移動先の容量は事前確認しません。失敗すると処理を停止し、完了済みのコピーは残します。代わりに直接削除することはありません。Finder の「元に戻す」は保証しません。`preview CONFIG --trash-dir PATH`・`apply CONFIG --trash-dir PATH` で明示的な退避先へ移動する場合は、宛先を実行前に表示・検証します。この退避先にはデスクトップのゴミ箱での復元用情報を記録しません。YAML に `trash_dir` 設定はありません。

example config は画像と動画をそれぞれ年月別のフォルダに copy する基本例です。metadata の編集には明示的な shell action を利用できます。

## Shell action

`shell` はコピー前の元ファイルを直接加工します。入力の絶対パスは環境変数 `TAMIS_FILE` で渡します。以下は、PNG のファイル名から撮影日時を書き込み、copy・trash する rule の一部です。ExifTool が必要なのはこの例を使う場合だけで、任意のインストール済みツールやスクリプトに置き換えられます。

```yaml
capture: '^.+? (?P<year>\d{4})_(?P<month>\d{2})_(?P<day>\d{2}) (?P<hour>\d{1,2})_(?P<minute>\d{2})_(?P<second>\d{2})\.png$'
actions:
  - shell:
      run: |
        exiftool -P -overwrite_original \
          "-EXIF:DateTimeOriginal=$CAPTURED" \
          "-EXIF:OffsetTimeOriginal=+09:00" \
          "$TAMIS_FILE"
      env:
        CAPTURED: "{{capture.year}}:{{capture.month}}:{{capture.day}} {{capture.hour | pad 2}}:{{capture.minute}}:{{capture.second}}"
  - copy: "~/Media/Archive/"
  - trash: true
```

`run` は `/bin/sh -eu -c` でそのまま実行し、テンプレート展開は `env` の値だけに行います。スクリプト内の環境変数は引用符で囲んでください。作業ディレクトリと環境変数を引き継ぎ、stdin は渡しません。実行上限は5分です。stdout は破棄し、失敗時は stderr の先頭16 KiB までをエラーに含めます。バックグラウンド処理は非対応で、終了・キャンセル時に process group を終了します。

`preview`・`explain` はコマンドと展開済み環境変数を表示し、実行しません。計画は shell 実行前の metadata を使います。撮影時刻を書き込んでも、その実行中の後続 rule の条件や宛先は再評価しません。必要に応じて、宛先テンプレートにもファイル名の capture を使ってください。

コマンド成功後、入力が引き続き通常ファイルであることを確認し、同一性・サイズ・mtime を更新して後続処理へ渡します。ファイルを置き換えるコマンドにも対応します。コマンド失敗時は停止しますが、コマンドが既に行った元ファイルの変更は巻き戻しません。後続 copy が skip された場合も、加工済みの元ファイルが残ります。shell だけの rule も利用できます。shell はユーザー権限で動き、sandbox ではないため、信頼できる設定だけを実行してください。

同じ固定の日時と offset の代入は、そのタグの値についてべき等です。`+=` などの加算はべき等ではありません。再実行でも filesystem 属性やファイルの同一性が変わる場合があり、例の `-P` は mtime を保持するために指定しています。[ExifTool の仕様](https://exiftool.org/exiftool_pod2.html)

## コマンド

```text
tamis scan PATH             metadata を NDJSON で出力
tamis preview CONFIG       ファイルを変更せず、検証済みの操作を表示
tamis apply CONFIG         操作を検証して実行
tamis explain CONFIG FILE  1ファイルの metadata、該当 rule、宛先を説明
tamis check CONFIG         設定を検証
tamis doctor CONFIG        provider とrule の location path を診断
```

設定ファイルは `CONFIG` として明示的に渡します。`~/.config` や `$XDG_CONFIG_HOME` からの自動読み込みは行いません。設定内の相対パスは、引き続き実行時の作業ディレクトリを基準に解決します。

`scan PATH` は設定なしでディレクトリをスキャンします。rule の location には `scan --config CONFIG` を使い、`PATH` とは併用しません。共通 flag は `--jobs` と `--json` です。

## 対応 metadata

- JPEG、TIFF、PNG、WebP、HEIC/HEIF、AVIF、DNG、CR2、NEF、PEF、ARW
- CR3、CRW、RW2
- QuickTime creation date、`©day`、embedded XML、`mvhd` を含む MOV、MP4
- PNG の `eXIf`、`tEXt`、`iTXt` creation date
- XMP、THM、同名 JPEG sidecar
- 最終 fallback として filesystem birth time と modification time

秒未満の精度は保証しません。CR3/CRW/RW2 以外の画像では、EXIF の `SubSecTimeOriginal`・`SubSecTimeDigitized` を撮影日時に合成せず、通常の EXIF 日時は秒単位で扱います。小数部を復元するための再解析は行いません。XMP や CR3/CRW/RW2 の decoder などが直接返す日時に小数秒が含まれる場合は、その値が残ることがあります。`raw` は decoder が返した値であり、小数秒の元の桁列を保持する保証はありません。連写の順序やファイル名の一意性を小数秒に依存させず、名前の衝突には `on_conflict` を使ってください。

動画抽出では `mdat` を読みません。metadata 抽出は上限付き worker pool で並列化し、式と正規表現は設定 load 時に一度だけ compile します。

XMP sidecar は 8 MiB、PNG text chunk は 1 MiB を上限とします。decoder の Read・Seek でキャンセルを確認するため、timeout は協調的な中断です。最終 fallback は引き続き filesystem birth time・mtime とし、rule の `captured_source` で区別できます。

`watch`、`undo`、backup、journal、組み込みの metadata writer、重複検出、cloud 連携は意図的に提供しません。

macOS と Linux を `CGO_ENABLED=0` で build できます。
