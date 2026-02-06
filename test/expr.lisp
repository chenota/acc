(in-package :acc/test)

(fiveam:def-suite expr)
(fiveam:in-suite expr)

(fiveam:test parse-int
             (let ((e (fiveam:finishes (expr-from "100"))))
               (when (fiveam:is (acc::int-node-p e))
                     (fiveam:is (= 100 (acc::int-node-value e))))))

(fiveam:test parse-ident
             (let ((e (fiveam:finishes (expr-from "x"))))
               (when (fiveam:is (acc::ident-node-p e))
                     (fiveam:is (string= "x" (acc::ident-node-name e))))))

(fiveam:test parse-expr-err (fiveam:signals error (expr-from "return")) 0)

(fiveam:test parse-cast-single (fiveam:is (acc::cast-node-p (fiveam:finishes (expr-from "(int) 100")))))

(fiveam:test parse-cast-multi (let ((e (fiveam:finishes (expr-from "(int64) (int32) (int16) (int8) 100"))))
                                (when (fiveam:is (acc::cast-node-p e))
                                      (fiveam:is (eq :int64 (acc::integer-type-size (acc::cast-node-cast-type e)))))))

(fiveam:test test-cast-missing-closing-paren (fiveam:signals error (expr-from "(int 100")) 0)

(fiveam:test test-cast-too-many-parens (fiveam:signals error (expr-from "((int)) 100")) 0)

(fiveam:test test-grouped-expr (fiveam:is (acc::int-node-p (fiveam:finishes (expr-from "((100))"))) 0))

(fiveam:test test-group-missing-paren (fiveam:signals error (expr-from "((100)")) 0)