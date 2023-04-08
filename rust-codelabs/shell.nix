{ pkgs ? import <nixpkgs> {} }:

with pkgs;

mkShell {
  buildInputs = [
    zsh
    rnix-lsp
    fd
    nodejs
    unzip
    gitui
    rustc
    rustfmt
    cargo
    libclang
    godot
    rust-analyzer
  ];
  MY_ENVIRONMENT_VARIABLE = "rust-codelabs";
}

