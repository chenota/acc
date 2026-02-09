(in-package :acc/test)

(fiveam:def-suite static-type)
(fiveam:in-suite static-type)

(fiveam:test program-basic (fiveam:finishes (typed-program-from "fun main int { return 0; }")))

(fiveam:test
  fun-nonstandard-return
  (let* ((f (fiveam:finishes (typed-fun-from "fun main int8 { return 0; }")))
         (s1 (first (acc::function-node-body f))))
    (when (fiveam:is (acc::return-statement-node-p s1))
          (fiveam:is (eq :int8 (acc::integer-type-size (acc::ast-node-type-info (acc::return-statement-node-expression s1))))))))

(fiveam:test return-context
  (let* ((s (fiveam:finishes (typed-stmt-from "return 0;" :env (acc::make-env :return-type (acc::make-integer-type :size :int32)))))
         (e (fiveam:finishes (acc::return-statement-node-expression s))))
    (when (fiveam:is (acc::int-node-p e) "The expression must be an integer.")
          (fiveam:is (eq :int32 (acc::integer-type-size (acc::ast-node-type-info e))) "The expression type must be an int32."))))

(fiveam:test no-return-context
  (fiveam:signals error (typed-stmt-from "return 0;")))

(fiveam:test untyped-integer
  (let ((e (typed-expr-from "0")))
    (when (fiveam:is (acc::int-node-p e))
          (fiveam:is (eq :generic (acc::integer-type-size (acc::ast-node-type-info e)))))))

(fiveam:test folded-cast-type
  (let ((e (typed-expr-from "(int8) 0")))
    (when (fiveam:is (acc::int-node-p e))
          (fiveam:is (eq :int8 (acc::integer-type-size (acc::ast-node-type-info e)))))))

(defmacro test-int-overflow-cast (size)
  `(fiveam:test ,(intern (format nil "OVERFLOW-INT~A" size)) (fiveam:signals error (typed-expr-from ,(format nil "(int~A) ~A" size (ash 1 size))))))

(test-int-overflow-cast 8)
(test-int-overflow-cast 16)
(test-int-overflow-cast 32)
(test-int-overflow-cast 64)

(fiveam:test
  int-type-from-var
  (let* ((f (fiveam:finishes (typed-fun-from "fun main int { let x : int = 5; x = 10; return x; }")))
         (s1 (first (acc::function-node-body f)))
         (s2 (second (acc::function-node-body f))))
    (when (fiveam:is (acc::declaration-node-p s1) "First statement must be a declaration")
          (fiveam:is (eq :int32 (acc::integer-type-size (acc::ast-node-type-info (acc::declaration-node-expression s1)))) "First expression must have an int type"))
    (when (fiveam:is (acc::assignment-node-p s2) "Second statement must be an assignment")
          (fiveam:is (eq :int32 (acc::integer-type-size (acc::ast-node-type-info (acc::assignment-node-expression s2)))) "Second expression must have an int32 type"))))

(fiveam:test assign-before-decl (fiveam:signals error (typed-fun-from "fun main int { let x : int = 5; y = 10; return x; }")))

(fiveam:test use-before-decl (fiveam:signals error (typed-fun-from "fun main int { let x : int = 5; x = y; return x; }")))

(fiveam:test return-needs-cast
  (let* ((f (fiveam:finishes (typed-fun-from "fun main int8 { let x : int = 5; return x; }")))
         (s2 (second (acc::function-node-body f))))
    (when (fiveam:is (acc::return-statement-node-p s2) "Final statement must be a return")
          (fiveam:is (acc::cast-node-p (acc::return-statement-node-expression s2)) "Return must be explicitly casted"))))

(fiveam:test declaration-needs-cast
  (let* ((f (fiveam:finishes (typed-fun-from "fun main int8 { let x : int = 5; let y : int8 = x; return y; }")))
         (s2 (second (acc::function-node-body f))))
    (when (fiveam:is (acc::declaration-node-p s2) "Second statement must be a declaration")
          (fiveam:is (acc::cast-node-p (acc::declaration-node-expression s2)) "Declaration must be explicitly casted"))))

(fiveam:test assignment-needs-cast
  (let* ((f (fiveam:finishes (typed-fun-from "fun main int8 { let x : int = 9999; let y : int8 = 0; y = x; return y; }")))
         (s3 (third (acc::function-node-body f))))
    (when (fiveam:is (acc::assignment-node-p s3) "Third statement must be an assignment")
          (fiveam:is (acc::cast-node-p (acc::assignment-node-expression s3)) "Assignment must be explicitly casted"))))