(in-package :acc/test)

(fiveam:def-suite static-type)
(fiveam:in-suite static-type)

(fiveam:test program-basic (fiveam:finishes (typed-program-from "fun main int { return 0; }")))

(fiveam:test fun-basic (fiveam:is (acc::function-type-p (acc::ast-node-type-info (fiveam:finishes (typed-fun-from "fun main int { return 0; }"))))))

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