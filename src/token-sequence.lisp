(in-package :acc)

(defclass token-sequence ()
    ((location :accessor location :initform 0 :type (integer 0 *))
     (token-list :accessor token-list :initarg :token-list)
     (end-row :accessor end-row :initform 0 :type (integer 0 *))
     (end-col :accessor end-col :initform 0 :type (integer 0 *)))
  (:documentation "Token sequence manager."))

(defmethod initialize-instance :after ((ts token-sequence) &key token-list &allow-other-keys)
  "Initialization logic for a token-sequence."
  (assert (typep token-list 'sequence))
  (assert (every #'token-p token-list))
  (setf (token-list ts) (coerce token-list 'vector))
  (when (> (length (token-list ts)) 0)
        (let ((last-token (aref
                              (token-list ts)
                            (1- (length (token-list ts))))))
          (setf (end-row ts) (+ (token-row last-token) (token-len last-token)))
          (setf (end-col ts) (token-col last-token)))))

(defmethod make-token-sequence (token-list)
  "Create a token sequence"
  (make-instance 'token-sequence :token-list token-list))

(defmethod peek ((ts token-sequence))
  "Return the token at the curren position. Returns an ENDMARKER if at the end of the sequence."
  (if (>= (location ts) (length (token-list ts)))
      (make-token :kind :ENDMARKER :row (end-row ts) :col (end-col ts) :len 0)
      (aref (token-list ts) (location ts))))

(defmethod advance ((ts token-sequence))
  "Advance the current position. Returns an ENDMARKER if at the end of the sequence."
  (let ((current-token (peek ts)))
    (unless (eq (token-kind current-token) :ENDMARKER) (incf (location ts)))
    current-token))