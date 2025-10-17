(asdf:defsystem :acc
  :description "Compiler for the acc language"
  :author "Alex Chenot"
  :license "MIT"
  :version "0.1.0"
  :serial t
  :components ((:file "package")
               (:module "src"
                        :components ((:file "lexer")))))