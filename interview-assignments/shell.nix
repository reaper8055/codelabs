{ pkgs ? import <nixpkgs> {} }:

with pkgs;

mkShell {
  buildInputs = [
    go
    zsh
    gotools
    gopls
    go-outline
    gocode
    gopkgs
    gocode-gomod
    godef
    golint
    rnix-lsp
    fd
    golangci-lint
    kind
    minikube
    kubectl
  ];
  MY_ENVIRONMENT_VARIABLE = "go-codelabs";
}

