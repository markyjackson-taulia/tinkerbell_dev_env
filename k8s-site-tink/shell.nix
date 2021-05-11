let _pkgs = import <nixpkgs> { };
in { pkgs ? import (_pkgs.fetchFromGitHub {
  owner = "NixOS";
  repo = "nixpkgs-channels";
  #branch@date: nixpkgs-unstable@2020-06-06
  rev = "029a5de08390bb03c3f44230b064fd1850c6658a";
  sha256 = "03fjkzhrs2avcvdabgm7a65rnyjaqbqdnv4q86qyjkkwg64g5m8x";
}) { } }:

with pkgs;

let
  kns = buildGoModule rec {
    pname = "kns";
    version = "unstable-20190927";

    src = fetchFromGitHub {
      owner = "blendle";
      repo = pname;
      rev = "7e5f78b781c6567f71068585f2b101caca99f0ef";
      sha256 = "1z461j8ixjm0bby4zh3h8y1y81njvirb68i2b8cn1fb49ls70hvy";
    };

    buildInputs = [ fzf kubectl ];
    installPhase = ''
      mkdir -p $out/bin
      install -m0755 bin/kns bin/ktx $out/bin
      ls -l $out/bin
    '';

  };

  kubelogin = buildGoModule rec {
    pname = "kubelogin";
    version = "1.19.2";

    goPackagePath = "github.com/int128/kubelogin";

    src = fetchFromGitHub {
      owner = "int128";
      repo = pname;
      rev = "v${version}";
      sha256 = "0c8sbffqx528ksl60qj0bpbzzl1wp75rbdmc5ydss0d0sycrzlpf";
    };

    vendorSha256 = "13ba6djd6ih5iaqh1l1w7nnk3dccxa9q6kn9kh3bs6mpgc3rmzv3";

    postInstall = ''
      ln -nsf ${pname} $out/bin/kubectl-oidc_login
    '';

  };
  krew = buildGoModule rec {
    pname = "krew";
    version = "0.3.4";

    goPackagePath = "github.com/kubernetes-sigs/krew";

    src = fetchFromGitHub {
      owner = "kubernetes-sigs";
      repo = pname;
      rev = "v${version}";
      sha256 = "0n10kpr2v9jzkz4lxrf1vf9x5zql73r5q1f1llwvjw6mb3xyn6ij";
    };

    vendorSha256 = "01jy73g087bng5xhbwd7pigsd44jl7pdfwf7ff43m9jsziknd39i";

    postInstall = ''
      ln -nsf ${pname} $out/bin/kubectl-${pname}
    '';

  };

in mkShell { buildInputs = [ kns kubectl kubelogin ]; }
