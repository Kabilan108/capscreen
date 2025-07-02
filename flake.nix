{
  description = "terminal-based screen recorder for x11";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };
  in {
    packages.${system}.default = pkgs.buildGoModule rec {
      pname = "capscreen";
      version = "latest";
      src = ./.;

      vendorHash = "sha256-HeEnr3GpFVvzGQOWZm4/Th5TKfU2Tyx756Hs3HhWv8g=";

      buildPhase = ''
        runHook preBuild
        make build
        runHook postBuild
      '';

      installPhase = ''
        runHook preInstall
        mkdir -p $out/bin
        cp build/capscreen $out/bin/
        runHook postInstall
      '';
    };
    devShells.${system}.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        self.packages.${system}.default
        go
        gopls
        ffmpeg
        xorg.xrandr
      ];
    };
  };
}
