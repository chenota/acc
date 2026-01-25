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
  (deftype integer-type-size () '(member :generic :int8 :int16 :int32 :int64))
  (defstruct (integer-type (:include type-base))
    (size nil :type integer-type-size))
  (defstruct (function-type (:include type-base))
    (parameters nil)
    (return-type nil)))