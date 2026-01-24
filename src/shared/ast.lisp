(in-package :acc)

(with-ignore-coverage
  ;; BASE
  (defstruct ast-node
    (type-info nil)
    (location nil))
  ;; EXPR
  (defstruct (cast-node (:include ast-node))
    (cast-type nil)
    (expression nil))
  (defstruct (int-node (:include ast-node))
    (value nil))
  ;;PROGRAM
  (defstruct (program-node (:include ast-node))
    (functions nil))
  (defstruct (function-node (:include ast-node))
    (name nil)
    (return-type nil)
    (body nil))
  (defstruct (return-statement-node (:include ast-node))
    (expression nil))
  ;;TYPE
  (defstruct (type-base (:constructor nil)))
  (defstruct (primitive-type (:include type-base))
    (kind nil :type keyword))
  (defstruct (function-type (:include type-base))
    (parameters nil)
    (return-type nil)))