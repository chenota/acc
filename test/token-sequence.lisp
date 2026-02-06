(in-package :acc/test)

(fiveam:def-suite token-sequence)
(fiveam:in-suite token-sequence)

(defmacro test-token-seq (name (&key (len 3)) &body body)
  `(fiveam:test ,name
                (let* ((seq-kind :k)
                       (seq-len ,len)
                       (seq
                        (fiveam:finishes
                         (acc::make-token-sequence
                          (loop for i from 0 below seq-len
                                collect (acc::make-token :kind seq-kind
                                                         :value i :loc (list 0 0) :len 0))))))
                  ,@body)))

(test-token-seq peek-start () (fiveam:is (= 0 (acc::token-value (acc::peek seq)))))

(test-token-seq peek-empty (:len 0) (fiveam:is (eq :ENDMARKER (acc::token-kind (acc::peek seq)))))

(test-token-seq advance-front ()
                (fiveam:is (= 0 (acc::token-value (acc::advance seq))) "First token must be 0")
                (fiveam:is (= 1 (acc::token-value (acc::peek seq))) "Next token must be 1"))

(test-token-seq advance-empty (:len 0) (fiveam:is (eq :ENDMARKER (acc::token-kind (acc::advance seq)))))

(test-token-seq capture () (fiveam:is (= 0 (acc::capture seq))))

(test-token-seq restore () (when (fiveam:finishes (acc::restore seq seq-len)) (fiveam:is (eq :ENDMARKER (acc::token-kind (acc::peek seq))))))

(test-token-seq restore-past-limit () (fiveam:signals error (acc::restore seq (1+ seq-len))))

(test-token-seq expect-exists () (fiveam:is (acc::token-p (acc::expect seq seq-kind))))

(test-token-seq expect-not-exists () (fiveam:is (null (acc::expect seq nil))))

(test-token-seq expect-with-value-exists () (fiveam:is (acc::token-p (acc::expect-with-value seq seq-kind 0))))

(test-token-seq expect-with-value-not-exists () (fiveam:is (null (acc::expect-with-value seq seq-kind "abc"))))