"""Add eye color column

Revision ID: 76b86e414aab
Revises: 6eb61f9873a9
Create Date: 2022-01-25 10:00:40.799166

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = '76b86e414aab'
down_revision = '6eb61f9873a9'
branch_labels = None
depends_on = None


def upgrade():
    op.add_column('cats', sa.Column('eye_color', sa.String))
    pass


def downgrade():
    op.drop_column('cats', 'eye_color')
    pass
