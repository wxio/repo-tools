{
  description = "repo-tools, the small programs the sheafdb repositories run on themselves";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      # The four platforms anyone here is plausibly on. Listed rather than
      # pulled from flake-utils: one fewer input to keep current, and the
      # set is short enough to read.
      systems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # One derivation, one binary: repo-tools, with the four tools
      # as subcommands. The module depends on github.com/jpillora/opts,
      # so vendorHash pins the hash of the vendored modules. Change
      # go.mod and this hash changes too: set it to lib.fakeHash, run
      # `nix build`, and copy the hash from the error.
      # go.mod asks for 1.27.1, and the default buildGoModule is a major
      # behind. buildGo127Module is the same builder with go_1_27 bound.
      mkTools = pkgs: pkgs.buildGo127Module {
        pname = "repo-tools";
        version = "0.1.0";
        src = ./.;
        vendorHash = "sha256-geV4Z117Ez1L46vM6dKKEnO13fxqi4aNdnrFo5v9kg8=";

        subPackages = [ "." ];

        meta = {
          description = "Markdown table formatter and prompt-history recorder for sheafdb";
          homepage = "https://github.com/wxio/repo-tools";
        };
      };
    in
    {
      # What a consuming repository takes. Put this derivation in its dev
      # shell and repo-tools is on PATH by name, so a prek hook and a
      # Claude Code hook each name a command rather than a path.
      packages = forAllSystems (pkgs: rec {
        repo-tools = mkTools pkgs;
        default = repo-tools;
      });

      # For working on the tools themselves. It does not hold the built
      # binary: `go run . <subcommand>` is the loop here.
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "repo-tools";

          packages = with pkgs; [
            go_1_27
            # The git hook runner. `prek install` wires prek.toml into
            # .git/hooks; `prek run -a` runs the hooks by hand.
            prek
            # The GitHub CLI, for pull requests and workflow runs.
            gh
          ];

          shellHook = ''
            echo "repo-tools"
            echo
            echo "  go test ./... && go vet ./...     the tests"
            echo "  go run . md-table-fmt -n *.md     align, report only"
            echo "  prek install                      the git hooks, once per clone"
            echo
          '';
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixpkgs-fmt);
    };
}
