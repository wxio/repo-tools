{
  description = "sheafdb-tools, the small programs the sheafdb repositories run on themselves";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      # The four platforms anyone here is plausibly on. Listed rather than
      # pulled from flake-utils: one fewer input to keep current, and the
      # set is short enough to read.
      systems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # One derivation for all four commands. The module depends on
      # nothing outside the standard library, so vendorHash is null and
      # there is no vendor directory to keep current.
      # go.mod asks for 1.27.1, and the default buildGoModule is a major
      # behind. buildGo127Module is the same builder with go_1_27 bound.
      mkTools = pkgs: pkgs.buildGo127Module {
        pname = "sheafdb-tools";
        version = "0.1.0";
        src = ./.;
        vendorHash = null;

        subPackages = [
          "cmd/md-table-fmt"
          "cmd/prompt-history-guard"
          "cmd/prompt-history-meta"
          "cmd/prompt-history-record"
        ];

        meta = {
          description = "Markdown table formatter and prompt-history recorder for sheafdb";
          homepage = "https://github.com/wxio/sheafdb-tools";
        };
      };
    in
    {
      # What a consuming repository takes. Put this derivation in its dev
      # shell and the four commands are on PATH by name, so a prek hook
      # and a Claude Code hook each name a command rather than a path.
      packages = forAllSystems (pkgs: rec {
        sheafdb-tools = mkTools pkgs;
        default = sheafdb-tools;
      });

      # For working on the tools themselves. It does not hold the built
      # commands: `go run ./cmd/<name>` is the loop here.
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "sheafdb-tools";

          packages = with pkgs; [
            go_1_27
            # The git hook runner. `prek install` wires prek.toml into
            # .git/hooks; `prek run -a` runs the hooks by hand.
            prek
            # The GitHub CLI, for pull requests and workflow runs.
            gh
          ];

          shellHook = ''
            echo "sheafdb-tools"
            echo
            echo "  go test ./... && go vet ./...     the tests"
            echo "  go run ./cmd/md-table-fmt -n .    align, report only"
            echo "  prek install                      the git hooks, once per clone"
            echo
          '';
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixpkgs-fmt);
    };
}
