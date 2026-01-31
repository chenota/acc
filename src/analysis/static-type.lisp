(in-package :acc)

(with-ignore-coverage
  (defstruct assign-type-environment
    (function-return-context nil))
  (defgeneric assign-type (node env)
    (:documentation "Assigns and returns the type of the node within the given environment."))
  (defgeneric in-bounds-p (node size)
    (:documentation "Checks if a node is in bounds with respect to type.")))

(defun set-program-types (ast)
  "Perform type analysis on ast."
  (assert (program-node-p ast))
  (assign-type ast (make-env))
  ast)

(defmethod assign-type ((node program-node) env)
  (loop for func in (program-node-functions node) do (assign-type func env))
  node)

(defmethod assign-type ((node function-node) env)
  (let ((inner-env (env-extend env)))
    (setf (env-return-type inner-env) (function-node-return-type node))
    (loop for stmt in (function-node-body node) do (assign-type stmt inner-env))
    (setf
      (function-node-type-info node)
      (make-function-type :parameters nil :return-type (function-node-return-type node)))
    node))

(defmethod assign-type ((node return-statement-node) env)
  (let ((e1 (assign-type (return-statement-node-expression node) env))
        (return-type (find-return-type env)))
    (unless
        (valid-cast-p e1 return-type)
      (error 'location-error
        :location (ast-node-location node)
        :message (format nil "Invalid return type: ~A" e1)))
    (setf (ast-node-type-info e1) return-type)
    (setf (return-statement-node-expression node) e1)
    node))

(defmethod assign-type ((node cast-node) env)
  (let ((e1 (assign-type (cast-node-expression node) env)))
    (unless (valid-cast-p e1 (cast-node-cast-type node))
      (error
          'location-error
        :location (ast-node-location node)
        :message (format nil "Invalid type cast: ~A to ~A" (ast-node-type-info e1) (cast-node-cast-type node))))
    (cond
     ;; Fold direct integer casts
     ((int-node-p e1)
       (setf (ast-node-type-info e1) (cast-node-cast-type node))
       e1)
     ;; Everything else must stay a cast node
     (t
       (setf (ast-node-type-info node) (cast-node-cast-type node))
       node))))

(defmethod assign-type ((node int-node) env)
  (setf (int-node-type-info node) (make-integer-type :size :generic))
  node)

(defun valid-cast-p (source-node destination-type)
  "Check if source can be type cast as destination."
  (cond
   ((int-node-p source-node) (in-bounds-p source-node destination-type))
   (t (error 'location-error :location (ast-node-location source-node) :message "Unknown type cast"))))

(defmethod in-bounds-p ((node int-node) size)
  (assert (integer-type-p size))
  (let ((v (int-node-value node))
        (s (integer-type-size size)))
    (alexandria:switch (s)
      (:int8 (<= (s-min 8) v (s-max 8)))
      (:int16 (<= (s-min 16) v (s-max 16)))
      (:int32 (<= (s-min 32) v (s-max 32)))
      (:int64 (<= (s-min 64) v (s-max 64)))
      (:generic t)
      (t nil))))

(defun s-max (n)
  "Maximum value of a singed integer with n bits."
  (1- (ash 1 (1- n))))

(defun s-min (n)
  "Minimum value of a signed integer with n bits."
  (- (ash 1 (1- n))))