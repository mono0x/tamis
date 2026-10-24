# Tamis

[English](README.md)

Tamis は、メタデータに基づく YAML ルールで画像・動画を整理する高速な CLI です。rename、copy、move、trash に対応し、1つの入力を複数の宛先へ振り分けられます。

`preview` で操作を確認し、`apply` で直接実行します。中間の計画ファイルは不要です。単一のバイナリとして動作し、純 Go のメタデータパーサーを使い、ExifTool のインストールは不要です。外部コマンドは明示的に設定した `shell` アクションでのみ実行します。

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

[GitHub Releases](https://github.com/mono0x/tamis/releases) から macOS / Linux とアーキテクチャ（`amd64` / `arm64`）に合うアーカイブをダウンロードし、展開した `tamis` を `PATH` に配置してください。各リリースに SHA-256 チェックサムを記載した `checksums.txt` が付属します。`tamis --version` でバージョンを確認できます。

ソースからインストールする場合は Go 1.27.3 以降が必要です:

```sh
go install github.com/mono0x/tamis/cmd/tamis@latest
```

## クイックスタート

[config.example.yaml](config.example.yaml) をコピーします。

```sh
cp config.example.yaml organize.yaml
```

`organize.yaml` の入力元・保存先パスを編集し、設定の検証と操作のプレビューを行います。

```sh
tamis check ./organize.yaml
tamis preview ./organize.yaml
```

`preview` は入力元をスキャンし、予定する操作を検証・表示します。ファイルは変更しません。抽出したメタデータ全体を確認するには、`tamis scan --config ./organize.yaml` を使います。

プレビューを確認したら実行します。

```sh
tamis apply ./organize.yaml
```

`apply` は再度スキャンして全操作を検証し、**確認を挟まず実行します**。実行内容は、その時点のファイルの状態に基づきます。既存の宛先は上書きせず、競合時の方針は `rename`・`skip`・`fail` から選びます。エラー時は停止し、完了済みのコピーや shell による変更は巻き戻しません。

## 設定

ルールの条件には [Expr](https://expr-lang.org/) を、パスには `{{ ... }}` テンプレートを使います。次の例は元ファイルを残し、画像を年月別のフォルダへコピーします。

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

アクションは `shell`・`copy`・`move`・`trash` です。アクションのテンプレートで使うメタデータは、同じルールの `when` で参照する必要があります。各項目・既定値・アクションの順序・テンプレートフィルターは [YAML 設定リファレンス](docs/configuration/README.ja.md) を参照してください。

## コマンド

```text
tamis scan PATH            メタデータを NDJSON で出力
tamis preview CONFIG       ファイルを変更せず、検証済みの操作を表示
tamis apply CONFIG         操作を検証して実行
tamis explain CONFIG FILE  1ファイルのメタデータ、該当ルール、宛先を説明
tamis check CONFIG         設定を検証
tamis doctor CONFIG        プロバイダーとルールの入力元を診断
```

設定ファイルは `CONFIG` として明示的に渡します。`~/.config` や `$XDG_CONFIG_HOME` からの自動読み込みは行いません。設定内の相対パスは、実行時の作業ディレクトリを基準に解決します。

`scan PATH` は設定なしでディレクトリをスキャンします。ルールの入力元には `scan --config CONFIG` を使い、`PATH` とは併用しません。共通フラグは `--jobs` と `--json` です。

## ドキュメント

| 目的                                                     | 文書                                                           |
| -------------------------------------------------------- | -------------------------------------------------------------- |
| ルール・条件式・テンプレートを書く                       | [YAML 設定リファレンス](docs/configuration/README.ja.md)       |
| スキャンの除外条件・実行時の保証・対応メタデータを調べる | [スキャン・実行・メタデータの仕様](docs/behavior/README.ja.md) |
| コピー前にメタデータを編集する                           | [Shell アクション](docs/configuration/README.ja.md#shell)      |
| OS 標準のゴミ箱や退避先を設定する                        | [Trash](docs/configuration/README.ja.md#trash)                 |
| ソースからビルド・テストする                             | [Development](docs/development/README.md)（英語）              |
| リリースの公開・アーカイブの検証を行う                   | [Releasing](docs/development/releasing.md)（英語）             |

JPEG、TIFF、PNG、WebP、HEIC/HEIF、AVIF、DNG、CR2、CR3、CRW、NEF、PEF、ARW、RW2、MOV、MP4 に対応し、サイドカーやファイルシステムの日時も利用します。詳細は [メタデータの対応範囲と制限](docs/behavior/README.ja.md#メタデータ) を参照してください。

`watch`・`undo`・バックアップ・ジャーナル・組み込みのメタデータ編集・重複検出・クラウド連携は提供しません。
